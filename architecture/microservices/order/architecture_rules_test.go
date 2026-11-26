// 架构守护测试 —— P4 微服务版（对应 Java 版 ArchitectureRulesTest 的 9 条规则）。
//
// Java 用 ArchUnit 扫描所有模块的 class 文件；Go 用 go/list 扫描 5 个 module 的
// package imports（workspace 下跨模块扫描）。规则映射：
//
//	domain_layers_do_not_depend_on_spring_or_jakarta   → TestDomainPurity（gin/gorm/grpc/protobuf）
//	order_does_not_touch_payment_or_product_internals  → TestOrderTouchesOnlyApiContracts
//	payment_and_product_do_not_depend_on_order         → TestProductAndPaymentDoNotDependOnOrder
//	product_and_payment_are_independent                → TestProductAndPaymentAreIndependent
//	rpc_clients_live_in_order_infrastructure_only      → TestRPCClientsLiveInOrderInfrastructureOnly
//	interfaces_do_not_access_infrastructure_or_impl    → TestInterfacesDoNotAccessInfrastructureOrImpl
//	domain_does_not_depend_on_application              → TestDomainPurity
//	application_does_not_depend_on_infrastructure      → TestApplicationDoesNotDependOnInfrastructure
//	domain_does_not_depend_on_infrastructure_or_interfaces → TestDomainPurity
//
// Go 版附加两条：shared 根包纯净（P2 传承的铁律）、gateway 生产代码不依赖业务服务。
// 组合根（各服务的 app/、cmd/）是部署单元特权层，不受层间规则约束。
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
	orderBase   = "github.com/jamieleestudio/go-tutorials/architecture/microservices/order"
	productBase = "github.com/jamieleestudio/go-tutorials/architecture/microservices/product"
	paymentBase = "github.com/jamieleestudio/go-tutorials/architecture/microservices/payment"
	sharedBase  = "github.com/jamieleestudio/go-tutorials/architecture/microservices/shared"
	gatewayBase = "github.com/jamieleestudio/go-tutorials/architecture/microservices/gateway"
)

// frameworkPrefixes 框架库前缀 —— 领域层禁依赖（对应 Java 的 spring/jakarta）。
var frameworkPrefixes = []string{
	"github.com/gin-gonic/gin",
	"gorm.io/",
	"google.golang.org/grpc",
	"google.golang.org/protobuf",
}

type pkgInfo struct {
	ImportPath string
	Imports    []string
}

// listModulePackages 在指定模块目录跑 go list -json ./...（跨模块扫描）。
// 只看 .Imports（非测试文件 imports）：测试文件（fake 注入、e2e 装配）天然豁免，
// 与 ArchUnit 扫描 main classes 的行为一致。
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
	for _, dir := range []string{".", "../product", "../payment", "../shared", "../gateway"} {
		for path, p := range listModulePackages(t, dir) {
			all[path] = p
		}
	}
	return all
}

func isInternal(path, base string) bool {
	return strings.HasPrefix(path, base+"/")
}

// --- 规则 1 / 7 / 9：领域层纯净 ---

func TestDomainPurity(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		isDomain := isInternal(path, productBase) || isInternal(path, paymentBase) || isInternal(path, orderBase)
		if !isDomain || !strings.Contains(path, "/domain") {
			continue
		}
		for _, imp := range p.Imports {
			for _, fw := range frameworkPrefixes {
				if strings.HasPrefix(imp, fw) {
					t.Errorf("%s imports framework %s", path, imp)
				}
			}
			// 规则 7：domain 不依赖 application
			if strings.Contains(imp, "/application") {
				t.Errorf("%s imports application %s", path, imp)
			}
			// 规则 9：domain 不依赖 infrastructure / interfaces（本服务或任何服务）
			if strings.Contains(imp, "/infrastructure") || strings.Contains(imp, "/interfaces") || strings.HasSuffix(imp, "/app") {
				t.Errorf("%s imports infrastructure/interfaces %s", path, imp)
			}
		}
	}
}

// --- 规则 2：order 只准依赖 product/payment 的 api 契约 + pb 包 ---

func TestOrderTouchesOnlyApiContracts(t *testing.T) {
	all := loadAll(t)
	allowed := map[string]bool{
		productBase + "/api":             true,
		productBase + "/proto/productpb": true,
		paymentBase + "/api":             true,
		paymentBase + "/proto/paymentpb": true,
	}
	for path, p := range all {
		if !isInternal(path, orderBase) {
			continue
		}
		for _, imp := range p.Imports {
			if isInternal(imp, productBase) && !allowed[imp] {
				t.Errorf("%s imports product internals %s (only api + pb allowed)", path, imp)
			}
			if isInternal(imp, paymentBase) && !allowed[imp] {
				t.Errorf("%s imports payment internals %s (only api + pb allowed)", path, imp)
			}
		}
	}
}

// --- 规则 3：product/payment 不依赖 order ---

func TestProductAndPaymentDoNotDependOnOrder(t *testing.T) {
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
		if isInternal(path, productBase) {
			for _, imp := range p.Imports {
				if isInternal(imp, paymentBase) {
					t.Errorf("%s imports payment %s", path, imp)
				}
			}
		}
		if isInternal(path, paymentBase) {
			for _, imp := range p.Imports {
				if isInternal(imp, productBase) {
					t.Errorf("%s imports product %s", path, imp)
				}
			}
		}
	}
}

// --- 规则 5：RPC 客户端只存在于 order 的 infrastructure ---

func TestRPCClientsLiveInOrderInfrastructureOnly(t *testing.T) {
	all := loadAll(t)
	clientPkg := orderBase + "/infrastructure/grpc"
	for path, p := range all {
		if !isInternal(path, orderBase) {
			continue
		}
		// 组合根（app/cmd）是部署单元特权层：它负责构造客户端并注入应用层
		if strings.HasSuffix(path, "/app") || strings.Contains(path, "/cmd/") {
			continue
		}
		if path == clientPkg {
			continue
		}
		for _, imp := range p.Imports {
			if imp == clientPkg {
				t.Errorf("%s imports rpc clients %s (only infrastructure may)", path, imp)
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

// --- 规则 8：application 不依赖 infrastructure ---

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

// --- 附加：shared 根包纯净（不依赖框架；persistence 子包例外，P2 传承） ---

func TestSharedKernelRootPurity(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		if path != sharedBase {
			continue
		}
		for _, imp := range p.Imports {
			for _, fw := range frameworkPrefixes {
				if strings.HasPrefix(imp, fw) {
					t.Errorf("shared root imports framework %s", imp)
				}
			}
		}
	}
}

// --- 附加：gateway 生产代码不依赖业务服务模块（对应 SCG 只路由不耦合） ---

func TestGatewayDoesNotDependOnBusinessServices(t *testing.T) {
	all := loadAll(t)
	for path, p := range all {
		if path != gatewayBase && !isInternal(path, gatewayBase) {
			continue
		}
		for _, imp := range p.Imports {
			if isInternal(imp, orderBase) || isInternal(imp, productBase) || isInternal(imp, paymentBase) {
				t.Errorf("%s imports business service %s", path, imp)
			}
		}
	}
}
