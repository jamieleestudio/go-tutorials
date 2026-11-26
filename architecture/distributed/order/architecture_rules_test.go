// 架构守护测试 —— P6 分布式版（对应 Java ArchitectureRulesTest 的 9 条规则）。
//
// Java 用 ArchUnit 扫描所有模块的 class 文件；Go 用 go/list 扫描 4 个 module 的
// package imports（workspace 下跨模块扫描）。规则 1:1 映射：
//
//	domain_layers_do_not_depend_on_spring_or_jakarta   → TestDomainDoesNotDependOnFrameworks
//	                                                     （gin/gorm/encoding-json）
//	order_does_not_touch_payment_or_product_internals  → TestOrderOnlyTouchesPaymentProductContracts
//	payment_and_product_do_not_depend_on_order         → TestPaymentAndProductDoNotDependOnOrder
//	product_and_payment_are_independent                → TestProductAndPaymentAreIndependent
//	rpc_clients_live_in_order_infrastructure_only      → TestRpcClientsLiveInOrderInfrastructureOnly
//	interfaces_do_not_access_infrastructure_or_impl    → TestInterfacesDoNotAccessInfrastructureOrImpl
//	domain_does_not_depend_on_application              → TestDomainDoesNotDependOnApplication
//	application_does_not_depend_on_infrastructure      → TestApplicationDoesNotDependOnInfrastructure
//	domain_does_not_depend_on_infrastructure_or_interfaces → TestDomainDoesNotDependOnInfrastructureOrInterfaces
//
// 关键规则一句话：order 与 payment/product 之间【只准经过 api 契约包】——
// RPC 客户端（infrastructure/rpc）是唯一知道 HTTP 细节的地方，application/domain
// 永远只认识 provider-defined 接口（"端口与适配器"在跨进程场景下的样子）。
//
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
	orderBase   = "github.com/jamieleestudio/go-tutorials/architecture/distributed/order"
	productBase = "github.com/jamieleestudio/go-tutorials/architecture/distributed/product"
	paymentBase = "github.com/jamieleestudio/go-tutorials/architecture/distributed/payment"
	sharedBase  = "github.com/jamieleestudio/go-tutorials/architecture/distributed/shared"
)

// frameworkPrefixes 框架库前缀 —— 领域层禁依赖（对应 Java 的 spring/jakarta）。
var frameworkPrefixes = []string{
	"github.com/gin-gonic/gin",
	"gorm.io/",
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

// --- 规则 1：领域层不依赖框架（对应 domain_layers_do_not_depend_on_spring_or_jakarta） ---

func TestDomainDoesNotDependOnFrameworks(t *testing.T) {
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
		}
	}
}

// --- 规则 2（P6 灵魂规则）：order 只准碰 payment/product 的 api 契约包 ---
// （对应 order_does_not_touch_payment_or_product_internals —— 跨进程边界 = api 包）

func TestOrderOnlyTouchesPaymentProductContracts(t *testing.T) {
	all := loadAll(t)
	allowed := map[string]bool{
		paymentBase + "/api": true,
		productBase + "/api": true,
	}
	for path, p := range all {
		if !isInternal(path, orderBase) {
			continue
		}
		for _, imp := range p.Imports {
			for _, foreign := range []string{paymentBase, productBase} {
				if isInternal(imp, foreign) && !allowed[imp] {
					t.Errorf("%s imports non-contract package of another service: %s (only */api is allowed)", path, imp)
				}
			}
		}
	}
}

// --- 规则 3：payment / product 不依赖 order ---

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

// --- 规则 4：product 与 payment 相互独立 ---

func TestProductAndPaymentAreIndependent(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		for _, imp := range p.Imports {
			if isInternal(path, productBase) && isInternal(imp, paymentBase) {
				t.Errorf("product package %s imports payment %s", path, imp)
			}
			if isInternal(path, paymentBase) && isInternal(imp, productBase) {
				t.Errorf("payment package %s imports product %s", path, imp)
			}
		}
	}
}

// --- 规则 5：RPC 客户端只住在 order/infrastructure/rpc ---
// （对应 rpc_clients_live_in_order_infrastructure_only —— "端口与适配器"的
// 适配器只在基础设施层：application/domain/interfaces 永不感知 HTTP 细节）

func TestRpcClientsLiveInOrderInfrastructureOnly(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		if !isInternal(path, orderBase) {
			continue
		}
		privileged := strings.Contains(path, "/application") ||
			strings.Contains(path, "/domain") ||
			strings.Contains(path, "/interfaces")
		if !privileged {
			continue
		}
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, orderBase+"/infrastructure/rpc") {
				t.Errorf("%s imports rpc clients %s (only order infrastructure may know the wire)", path, imp)
			}
		}
	}
}

// --- 规则 6：interfaces 不依赖 infrastructure / impl ---

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

// --- 规则 7：domain 不依赖 application ---

func TestDomainDoesNotDependOnApplication(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		for _, base := range []string{orderBase, productBase, paymentBase} {
			if !isInternal(path, base) || !strings.Contains(path, "/domain") {
				continue
			}
			for _, imp := range p.Imports {
				if strings.Contains(imp, "/application") {
					t.Errorf("%s imports application %s", path, imp)
				}
			}
		}
	}
}

// --- 规则 8：application 不依赖 infrastructure（依赖倒置：只认识端口） ---

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

// --- 规则 9：domain 不依赖 infrastructure / interfaces ---

func TestDomainDoesNotDependOnInfrastructureOrInterfaces(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		for _, base := range []string{orderBase, productBase, paymentBase} {
			if !isInternal(path, base) || !strings.Contains(path, "/domain") {
				continue
			}
			for _, imp := range p.Imports {
				if strings.Contains(imp, "/infrastructure") || strings.Contains(imp, "/interfaces") || strings.HasSuffix(imp, "/app") {
					t.Errorf("%s imports infrastructure/interfaces %s", path, imp)
				}
			}
		}
	}
}

// --- 附加：契约包纯净（只依赖 shared —— 它是跨进程的唯一窗口） ---

func TestContractPackagesOnlyDependOnShared(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		contract := path == productBase+"/api" || path == paymentBase+"/api"
		if !contract {
			continue
		}
		for _, imp := range p.Imports {
			if strings.Contains(imp, "/domain") || strings.Contains(imp, "/application") ||
				strings.Contains(imp, "/infrastructure") || strings.Contains(imp, "/interfaces") {
				t.Errorf("contract package %s imports internal package %s", path, imp)
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
			for _, fw := range frameworkPrefixes[:2] { // shared 根包只禁 gin/gorm，不禁 encoding/json
				if strings.HasPrefix(imp, fw) {
					t.Errorf("shared root imports framework %s", imp)
				}
			}
		}
	}
}
