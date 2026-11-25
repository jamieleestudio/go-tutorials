// 架构守护测试 —— 多模块版。
//
// 与 P1 的关键差异（对应 Java 版注释 "context boundaries are now Maven modules"）：
//
//   - 上下文边界已经从"包约定"升级为【Go module 边界】：go.mod 的 require 列表
//     是【编译期强制】的 —— order 模块没 require product 模块的话，
//     import "…/product/…" 直接编译失败。ArchUnit 挡不住的越界，go build 就挡住了。
//   - 因此本文件分两层守护：
//       1. go.mod require 规则 —— 模块级依赖方向（新增，最强约束）
//       2. go list import 规则 —— 包级分层规则（同 P1 / Java 版 ArchUnit）
//   - Java 版的 bounded_contexts_are_free_of_cycles 规则在 Go 里【免费获得】：
//     Go 编译器本身禁止任何 import 环，想写个循环依赖都编译不过。
package bootstrap_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const moduleBase = "github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module"

// 内部模块清单（module path 末段）
var internalModules = []string{"shared", "product", "payment", "order", "bootstrap"}

func modulePath(mod string) string { return moduleBase + "/" + mod }

func moduleDir(t *testing.T, mod string) string {
	t.Helper()
	return filepath.Join("..", mod) // 测试工作目录是 bootstrap/，各模块是其同级目录
}

// --- 第一层：go.mod require 规则（编译期依赖声明） ---

// parseRequire 解析 go.mod 中直接 require 的本仓库内部模块集合。
func parseRequire(t *testing.T, mod string) map[string]bool {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleDir(t, mod), "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod of %s: %v", mod, err)
	}
	defer func() { _ = f.Close() }()

	requireLine := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(moduleBase) + `/([a-z]+)\s`)
	deps := make(map[string]bool)
	inRequire := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if m := requireLine.FindStringSubmatch(line); m != nil {
			deps[m[1]] = true
		}
	}
	return deps
}

func TestModuleDependencyRules(t *testing.T) {
	cases := []struct {
		mod      string
		want     []string              // 必须依赖的模块
		forbid   []string              // 禁止依赖的模块
		exactly  []string              // 精确等于（既不许多也不许少）
		exactSet bool
	}{
		{mod: "shared", exactly: []string{}, exactSet: true}, // 内核不依赖任何业务模块
		{mod: "product", exactly: []string{"shared"}, exactSet: true},
		{mod: "payment", exactly: []string{"shared"}, exactSet: true},
		{mod: "order", exactly: []string{"shared", "product", "payment"}, exactSet: true},
		{mod: "bootstrap", forbid: []string{}}, // 部署单元：require 全部（下面单独校验）
	}
	for _, tc := range cases {
		t.Run(tc.mod, func(t *testing.T) {
			deps := parseRequire(t, tc.mod)
			if tc.exactSet {
				want := make(map[string]bool)
				for _, w := range tc.exactly {
					want[w] = true
				}
				if len(deps) != len(want) {
					t.Fatalf("%s 的 go.mod require 了 %v，期望恰好 %v", tc.mod, keys(deps), tc.exactly)
				}
				for w := range want {
					if !deps[w] {
						t.Errorf("%s 缺少对 %s 的依赖", tc.mod, w)
					}
				}
				for d := range deps {
					if !want[d] {
						t.Errorf("%s 出现了非预期的依赖 %s", tc.mod, d)
					}
				}
				return
			}
			for _, f := range tc.forbid {
				if deps[f] {
					t.Errorf("%s 不得依赖 %s", tc.mod, f)
				}
			}
		})
	}

	// bootstrap 必须依赖全部业务模块（部署单元）
	deps := parseRequire(t, "bootstrap")
	for _, m := range []string{"shared", "product", "payment", "order"} {
		if !deps[m] {
			t.Errorf("bootstrap 必须依赖 %s（部署单元特权）", m)
		}
	}
}

// --- 第二层：go list import 规则（包级分层，跨全部模块） ---

type goPackage struct {
	ImportPath string
	Imports    []string
}

// listAllPackages 对全部 5 个模块分别跑 go list，合并结果。
func listAllPackages(t *testing.T) map[string]goPackage {
	t.Helper()
	pkgs := make(map[string]goPackage)
	for _, mod := range internalModules {
		cmd := exec.Command("go", "list", "-json", "./...")
		cmd.Dir = moduleDir(t, mod)
		cmd.Stderr = os.Stderr
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("go list failed in %s: %v", mod, err)
		}
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
	}
	return pkgs
}

// moduleOf 返回 import path 所属的内部模块名，不属于本仓库则返回 ""。
func moduleOf(importPath string) string {
	for _, m := range internalModules {
		if importPath == modulePath(m) || strings.HasPrefix(importPath, modulePath(m)+"/") {
			return m
		}
	}
	return ""
}

// layerOf 返回包所属的层（domain/application/impl/infrastructure/interfaces），无则 ""。
func layerOf(importPath string) string {
	for _, layer := range []string{"domain", "application", "impl", "infrastructure", "interfaces"} {
		for _, p := range strings.Split(strings.TrimPrefix(importPath, moduleBase+"/"), "/") {
			if p == layer {
				return layer
			}
		}
	}
	return ""
}

func hasSegment(path, seg string) bool {
	for _, p := range strings.Split(path, "/") {
		if p == seg {
			return true
		}
	}
	return false
}

func isInternalModule(importPath string) bool { return moduleOf(importPath) != "" }

func isThirdParty(importPath string) bool {
	first := importPath
	if i := strings.Index(importPath, "/"); i >= 0 {
		first = importPath[:i]
	}
	return strings.Contains(first, ".")
}

func allowedDomainDeps(importPath string) bool {
	switch {
	case strings.HasPrefix(importPath, "github.com/shopspring/decimal"),
		strings.HasPrefix(importPath, "github.com/google/uuid"):
		return true
	case importPath == modulePath("shared"): // shared 内核根包
		return true
	}
	return false
}

func TestDomainLayerPurity(t *testing.T) {
	for path, p := range listAllPackages(t) {
		if layerOf(path) != "domain" {
			continue
		}
		for _, imp := range p.Imports {
			if l := layerOf(imp); l != "" && l != "domain" {
				t.Errorf("domain 包 %s 不得依赖 %s 层，但导入了 %s", path, l, imp)
			}
			if isThirdParty(imp) && !allowedDomainDeps(imp) {
				// 放行同模块的 domain 子包（如 order/domain/event）
				if !(isInternalModule(imp) && moduleOf(imp) == moduleOf(path)) {
					t.Errorf("domain 包 %s 只允许标准库/decimal/uuid/shared 依赖，但导入了 %s", path, imp)
				}
			}
		}
	}
}

func TestInterfacesLayerDoesNotAccessInfrastructureOrImpl(t *testing.T) {
	for path, p := range listAllPackages(t) {
		if layerOf(path) != "interfaces" {
			continue
		}
		for _, imp := range p.Imports {
			if layerOf(imp) == "infrastructure" {
				t.Errorf("interfaces 包 %s 不得访问 infrastructure，但导入了 %s", path, imp)
			}
			if layerOf(imp) == "impl" {
				t.Errorf("interfaces 包 %s 只能注入 Service 接口，不得访问 application/impl，但导入了 %s", path, imp)
			}
		}
	}
}

func TestInfrastructureAndImplDoNotAccessInterfaces(t *testing.T) {
	for path, p := range listAllPackages(t) {
		layer := layerOf(path)
		if layer != "infrastructure" && layer != "impl" {
			continue
		}
		for _, imp := range p.Imports {
			if layerOf(imp) == "interfaces" {
				t.Errorf("%s 包 %s 不得访问 interfaces，但导入了 %s", layer, path, imp)
			}
		}
	}
}

// order 模块只能使用 product/payment 的 application 层接口。
// 注意：这条规则同样由 go.mod require 保证 —— 不过 require 只到模块级，
// 这里的 import 分析把约束细化到【对方模块的哪一层】。
func TestOrderModuleOnlyUsesProductPaymentApplicationLayer(t *testing.T) {
	for path, p := range listAllPackages(t) {
		if moduleOf(path) != "order" {
			continue
		}
		for _, imp := range p.Imports {
			other := moduleOf(imp)
			if other == "product" || other == "payment" {
				if l := layerOf(imp); l != "application" && l != "domain" && l == "" {
					t.Errorf("order 包 %s 依赖了 %s 模块的非法层包：%s（只允许 application）", path, other, imp)
				}
				if l := layerOf(imp); l == "domain" || l == "infrastructure" || l == "interfaces" {
					t.Errorf("order 包 %s 依赖了 %s 模块的 %s 层（只允许 application）：%s", path, other, l, imp)
				}
			}
		}
	}
}

func TestProductAndPaymentDoNotDependOnOrder(t *testing.T) {
	for path, p := range listAllPackages(t) {
		mod := moduleOf(path)
		if mod != "product" && mod != "payment" {
			continue
		}
		for _, imp := range p.Imports {
			if moduleOf(imp) == "order" {
				t.Errorf("%s（%s 模块）不得依赖 order 模块：%s", path, mod, imp)
			}
		}
	}
}

func TestProductAndPaymentAreIndependent(t *testing.T) {
	for path, p := range listAllPackages(t) {
		mod := moduleOf(path)
		if mod != "product" && mod != "payment" {
			continue
		}
		for _, imp := range p.Imports {
			other := moduleOf(imp)
			// 只禁止业务上下文互指；依赖 shared 内核是合法的
			if (other == "product" || other == "payment") && other != mod {
				t.Errorf("%s（%s 模块）不得依赖 %s 模块", path, mod, other)
			}
		}
	}
}

// shared 内核根包保持纯净：不依赖任何业务模块、不依赖 web 框架。
// （shared/persistence 子包允许依赖 gorm —— 它是 Go 版的"事务传播 starter"，
//   对应 Java 版由 spring-tx 提供的运行时能力。）
func TestSharedKernelPurity(t *testing.T) {
	for path, p := range listAllPackages(t) {
		if path != modulePath("shared") {
			continue
		}
		for _, imp := range p.Imports {
			if moduleOf(imp) != "" {
				t.Errorf("shared 内核不得依赖任何业务模块：%s", imp)
			}
			if strings.HasPrefix(imp, "github.com/gin-gonic/gin") {
				t.Errorf("shared 内核不得依赖 web 框架：%s", imp)
			}
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
