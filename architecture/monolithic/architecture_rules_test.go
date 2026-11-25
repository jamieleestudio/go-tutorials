// 架构守护测试 —— ArchUnit 的 Go 对应物。
//
// Java 版用 ArchUnit 扫描字节码检查依赖规则；Go 生态没有等价的成熟工具，
// 这里用 `go list -json ./...` 输出每个包的导入列表，自行断言规则。
// 规则与 Java 版 ArchitectureRulesTest 一一对应。
//
// 好处：分层被破坏时 CI 直接红 —— 架构不再是口头约定。
package monolithic_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type goPackage struct {
	ImportPath string
	Imports    []string
}

// listPackages 用 go list 枚举本模块所有包及其导入（不含测试代码的导入）。
func listPackages(t *testing.T) map[string]goPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list failed: %v", err)
	}

	pkgs := make(map[string]goPackage)
	dec := json.NewDecoder(&out)
	for {
		var p goPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("parse go list output: %v", err)
		}
		pkgs[p.ImportPath] = p
	}
	return pkgs
}

// hasSegment 判断 import path 是否包含某个路径段。
// 对应 ArchUnit 的 "..order.domain.." 通配匹配。
func hasSegment(path, seg string) bool {
	for _, p := range strings.Split(path, "/") {
		if p == seg {
			return true
		}
	}
	return false
}

// contextOf 返回包所属的限界上下文（order/product/payment），不属于任何上下文则返回 ""。
func contextOf(path string) string {
	for _, c := range []string{"order", "product", "payment"} {
		if hasSegment(path, c) {
			return c
		}
	}
	return ""
}

func isThirdParty(importPath string) bool {
	first := importPath
	if i := strings.Index(importPath, "/"); i >= 0 {
		first = importPath[:i]
	}
	return strings.Contains(first, ".") // 标准库首段没有点：context、fmt、gorm.io、github.com...
}

const modulePrefix = "github.com/jamieleestudio/go-tutorials/architecture/monolithic/"

func isInternalModule(importPath string) bool {
	return strings.HasPrefix(importPath, modulePrefix)
}

func allowedDomainDeps(importPath string) bool {
	// 领域层允许的第三方库：decimal（BigDecimal 对应物）、uuid
	switch {
	case strings.HasPrefix(importPath, "github.com/shopspring/decimal"),
		strings.HasPrefix(importPath, "github.com/google/uuid"):
		return true
	// 允许的内部包：shared 内核
	case isInternalModule(importPath) && hasSegment(importPath, "shared"):
		return true
	}
	return false
}

// --- 领域纯净性 ---

func TestDomainLayerPurity(t *testing.T) {
	for path, p := range listPackages(t) {
		if !hasSegment(path, "domain") {
			continue
		}
		for _, imp := range p.Imports {
			if hasSegment(imp, "application") || hasSegment(imp, "infrastructure") || hasSegment(imp, "interfaces") {
				t.Errorf("domain 包 %s 不得依赖应用/基础设施/接口层，但导入了 %s", path, imp)
			}
			if isThirdParty(imp) && !allowedDomainDeps(imp) {
				// 放行同上下文的 domain 子包（如 order/domain/event）
				if !(isInternalModule(imp) && contextOf(imp) == contextOf(path)) {
					t.Errorf("domain 包 %s 只允许标准库/decimal/uuid/shared 依赖，但导入了 %s", path, imp)
				}
			}
		}
	}
}

// --- 分层依赖方向 ---

func TestInterfacesLayerDoesNotAccessInfrastructureOrImpl(t *testing.T) {
	for path, p := range listPackages(t) {
		if !hasSegment(path, "interfaces") {
			continue
		}
		for _, imp := range p.Imports {
			if hasSegment(imp, "infrastructure") {
				t.Errorf("interfaces 包 %s 不得访问 infrastructure，但导入了 %s", path, imp)
			}
			if hasSegment(imp, "impl") {
				t.Errorf("interfaces 包 %s 只能注入 Service 接口，不得访问 application/impl，但导入了 %s", path, imp)
			}
		}
	}
}

func TestInfrastructureLayerDoesNotAccessInterfaces(t *testing.T) {
	for path, p := range listPackages(t) {
		if !hasSegment(path, "infrastructure") {
			continue
		}
		for _, imp := range p.Imports {
			if hasSegment(imp, "interfaces") {
				t.Errorf("infrastructure 包 %s 不得访问 interfaces，但导入了 %s", path, imp)
			}
		}
	}
}

func TestImplDoesNotAccessInterfaces(t *testing.T) {
	for path, p := range listPackages(t) {
		if !hasSegment(path, "impl") {
			continue
		}
		for _, imp := range p.Imports {
			if hasSegment(imp, "interfaces") {
				t.Errorf("application/impl 包 %s 不得访问 interfaces，但导入了 %s", path, imp)
			}
		}
	}
}

// --- 限界上下文隔离 ---

func TestInfrastructureDoesNotCrossContexts(t *testing.T) {
	for path, p := range listPackages(t) {
		if !hasSegment(path, "infrastructure") {
			continue
		}
		own := contextOf(path)
		if own == "" {
			continue
		}
		for _, imp := range p.Imports {
			other := contextOf(imp)
			if other != "" && other != own && hasSegment(imp, "infrastructure") {
				t.Errorf("%s 的 infrastructure 不得跨上下文访问 %s 的 infrastructure", path, other)
			}
		}
	}
}

func TestOrderContextOnlyUsesProductPaymentApplicationLayer(t *testing.T) {
	for path, p := range listPackages(t) {
		if contextOf(path) != "order" {
			continue
		}
		for _, imp := range p.Imports {
			other := contextOf(imp)
			if (other == "product" || other == "payment") &&
				(hasSegment(imp, "domain") || hasSegment(imp, "infrastructure")) {
				t.Errorf("order 包 %s 依赖了 %s 上下文的 domain/infrastructure 层（只允许 application 接口）：%s", path, other, imp)
			}
		}
	}
}

func TestProductAndPaymentDoNotDependOnOrder(t *testing.T) {
	for path, p := range listPackages(t) {
		c := contextOf(path)
		if c != "product" && c != "payment" {
			continue
		}
		for _, imp := range p.Imports {
			if contextOf(imp) == "order" {
				t.Errorf("%s（%s 上下文）不得依赖 order 上下文：%s", path, c, imp)
			}
		}
	}
}

func TestProductAndPaymentAreIndependent(t *testing.T) {
	for path, p := range listPackages(t) {
		c := contextOf(path)
		if c != "product" && c != "payment" {
			continue
		}
		for _, imp := range p.Imports {
			other := contextOf(imp)
			if other != "" && other != c {
				t.Errorf("%s（%s 上下文）不得依赖 %s 上下文", path, c, other)
			}
		}
	}
}

// --- 共享内核 ---

func TestSharedKernelDoesNotDependOnContexts(t *testing.T) {
	for path, p := range listPackages(t) {
		if !hasSegment(path, "shared") || contextOf(path) != "" {
			continue
		}
		for _, imp := range p.Imports {
			if c := contextOf(imp); c != "" {
				t.Errorf("shared 内核包 %s 不得依赖任何业务上下文（%s）：%s", path, c, imp)
			}
		}
	}
}
