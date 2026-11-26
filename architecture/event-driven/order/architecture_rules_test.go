// 架构守护测试 —— P5 事件驱动版（对应 Java 版 ArchitectureRulesTest 的 5 条规则）。
//
// Java 用 ArchUnit 扫描所有模块的 class 文件；Go 用 go/list 扫描 4 个 module 的
// package imports（workspace 下跨模块扫描）。规则映射：
//
//	domain_layers_are_pure（禁 Spring/jackson/内部层） → TestDomainPurity
//	                                                   （gin/gorm/grpc/protobuf/encoding-json）
//	application_does_not_depend_on_infrastructure      → TestApplicationDoesNotDependOnInfrastructure
//	interfaces_do_not_access_infrastructure_or_impl    → TestInterfacesDoNotAccessInfrastructureOrImpl
//	outbox_lives_in_infrastructure_only                → TestOutboxLivesInInfrastructureOnly
//	payment_and_product_do_not_depend_on_order         → TestPaymentAndProductDoNotDependOnOrder
//
// Go 版附加：shared 根包纯净（P2 传承铁律）。
// 组合根（各服务的 app/、cmd/）是部署单元特权层，不受层间规则约束；
// 测试文件（fake 注入、e2e 装配）天然豁免 —— 只看 .Imports（非测试 imports），
// 与 ArchUnit 扫描 main classes 的行为一致。
package order_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const (
	orderBase   = "github.com/jamieleestudio/go-tutorials/architecture/event-driven/order"
	productBase = "github.com/jamieleestudio/go-tutorials/architecture/event-driven/product"
	paymentBase = "github.com/jamieleestudio/go-tutorials/architecture/event-driven/payment"
	sharedBase  = "github.com/jamieleestudio/go-tutorials/architecture/event-driven/shared"
)

// frameworkPrefixes 框架库前缀 —— 领域层禁依赖（对应 Java 的 spring/jakarta/jackson）。
var frameworkPrefixes = []string{
	"github.com/gin-gonic/gin",
	"gorm.io/",
	"google.golang.org/grpc",
	"google.golang.org/protobuf",
	"encoding/json", // 对应 Java 的禁 jackson：领域层不感知 JSON
}

type pkgInfo struct {
	ImportPath string
	Imports    []string
}

// listModulePackages 在指定模块目录跑 go list -json ./...（跨模块扫描）。
func listModulePackages(t *testing.T, dir string) map[string]pkgInfo {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list %s: %v", dir, err)
	}

	pkgs := make(map[string]pkgInfo)
	dec := json.NewDecoder(&out)
	for {
		var p pkgInfo
		if err := dec.Decode(&p); err != nil {
			break
		}
		pkgs[p.ImportPath] = p
	}
	return pkgs
}

func loadAll(t *testing.T) map[string]pkgInfo {
	t.Helper()
	all := make(map[string]pkgInfo)
	for _, dir := range []string{".", "../product", "../payment", "../shared"} {
		for path, p := range listModulePackages(t, dir) {
			all[path] = p
		}
	}
	return all
}

func isInternal(path, base string) bool {
	return strings.HasPrefix(path, base+"/")
}

func isCombinationRoot(path string) bool {
	// 组合根是部署单元特权层：装配、接线、选实现都在这里发生。
	return strings.HasSuffix(path, "/app") || strings.Contains(path, "/cmd/")
}

// --- 规则 1：领域层纯净（禁框架/JSON、禁 application/infrastructure/interfaces） ---

func TestDomainPurity(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		isDomain := (isInternal(path, productBase) || isInternal(path, paymentBase) || isInternal(path, orderBase)) &&
			strings.Contains(path, "/domain")
		if !isDomain {
			continue
		}
		for _, imp := range p.Imports {
			for _, fw := range frameworkPrefixes {
				if strings.HasPrefix(imp, fw) {
					t.Errorf("%s imports framework %s", path, imp)
				}
			}
			if strings.Contains(imp, "/application") {
				t.Errorf("%s imports application %s", path, imp)
			}
			if strings.Contains(imp, "/infrastructure") || strings.Contains(imp, "/interfaces") || strings.HasSuffix(imp, "/app") {
				t.Errorf("%s imports infrastructure/interfaces %s", path, imp)
			}
		}
	}
}

// --- 规则 2：application 不依赖 infrastructure（依赖倒置：只认识端口） ---

func TestApplicationDoesNotDependOnInfrastructure(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		for _, base := range []string{orderBase, productBase, paymentBase} {
			if !isInternal(path, base) || !strings.Contains(path, "/application") {
				continue
			}
			for _, imp := range p.Imports {
				if strings.HasPrefix(imp, base+"/infrastructure") {
					t.Errorf("%s imports infrastructure %s", path, imp)
				}
			}
		}
	}
}

// --- 规则 3：interfaces 不依赖 infrastructure / impl ---

func TestInterfacesDoNotAccessInfrastructureOrImpl(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		for _, base := range []string{orderBase, productBase, paymentBase} {
			if !isInternal(path, base) || !strings.Contains(path, "/interfaces") {
				continue
			}
			for _, imp := range p.Imports {
				if strings.HasPrefix(imp, base+"/infrastructure") || strings.HasPrefix(imp, base+"/application/impl") {
					t.Errorf("%s imports %s", path, imp)
				}
			}
		}
	}
}

// --- 规则 4：Outbox 只属于 infrastructure（发布/投递是部署细节） ---

func TestOutboxLivesInInfrastructureOnly(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		for _, base := range []string{orderBase, productBase, paymentBase} {
			if !isInternal(path, base) || isCombinationRoot(path) {
				continue
			}
			inInfra := strings.HasPrefix(path, base+"/infrastructure")
			for _, imp := range p.Imports {
				isOutboxImport := strings.Contains(imp, "/infrastructure/outbox") ||
					// processed 去重表同样是持久化细节
					strings.Contains(imp, "/infrastructure/processed")
				if isOutboxImport && !inInfra && !isCombinationRoot(path) {
					t.Errorf("%s imports outbox/processed outside infrastructure: %s", path, imp)
				}
			}
		}
	}
}

// --- 规则 5：payment / product 不依赖 order（Saga 只靠事件协同，不反向依赖） ---

func TestPaymentAndProductDoNotDependOnOrder(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		if isInternal(path, productBase) || isInternal(path, paymentBase) {
			for _, imp := range p.Imports {
				if isInternal(imp, orderBase) {
					t.Errorf("%s imports order %s", path, imp)
				}
			}
		}
	}
}

// --- 附加：shared 根包纯净（不依赖框架；persistence 子包例外，P2 传承） ---

func TestSharedKernelRootPurity(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		if path != sharedBase {
			continue
		}
		for _, imp := range p.Imports {
			for _, fw := range frameworkPrefixes[:4] { // shared 根包只禁框架，不禁 encoding/json
				if strings.HasPrefix(imp, fw) {
					t.Errorf("shared root imports framework %s", imp)
				}
			}
		}
	}
}
