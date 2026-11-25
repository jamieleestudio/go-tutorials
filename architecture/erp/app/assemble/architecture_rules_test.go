// 架构守护测试 —— erp 版（对应 Java 版 erp-architecture-test 模块的 4 个测试类）。
//
// 守护规则与 Java 版对照：
//
//	LayerRulesTest           → TestLayeredDependencies / TestApiPackagesDoNotDependOnImplementationLayers
//	ApiContractRulesTest     → TestApiInterfacesAreImplementedOnlyByProviders（go/types 类型检查器实现）
//	DomainIsolationRulesTest → TestCrossDomainDependenciesUseApiPackageOnly
//	                           （domains_free_of_cycles 由 Go 编译器免费保证 —— import 环无法编译）
//	DomainPurityRulesTest    → TestDomainPurity
//	EndIsolationRulesTest    → TestEndsDoNotDependOnEachOther
//	PlatformRulesTest        → TestPlatformDoesNotDependOnBusinessDomains / TestSharedKernelPurity /
//	                             TestSystemDomainIsPureProvider
//
// 与 P2 的差异：单 module 内边界全靠 import 分析（Java 版也是单 jar + ArchUnit，语义最近）。
package assemble_test

import (
	"bytes"
	"encoding/json"
	"go/types"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/stretchr/testify/require"
)

const moduleBase = "github.com/jamieleestudio/go-tutorials/architecture/erp"

var domains = []string{"system", "grade", "attendance"}

// 端列表（interfaces/<end>；provider 也是 interfaces 成员但不属于"端"——
// Java 版 excludeFilters 的正则里同样没有 provider）。
var ends = []string{"web", "admin", "mobile", "openapi", "internalapi", "device", "mq", "job"}

type goPackage struct {
	ImportPath string
	Imports    []string
}

func listModulePackages(t *testing.T) map[string]goPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = "../.." // 测试工作目录是 app/assemble，erp 模块根是其上两级
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	require.NoError(t, cmd.Run(), "go list failed")

	pkgs := make(map[string]goPackage)
	dec := json.NewDecoder(&out)
	for {
		var p goPackage
		if err := dec.Decode(&p); err != nil {
			break
		}
		pkgs[p.ImportPath] = p
	}
	return pkgs
}

// topOf 返回域内首个段：system / grade / attendance / platform / app / shared。
func topOf(importPath string) string {
	rel := strings.TrimPrefix(importPath, moduleBase+"/")
	if rel == importPath {
		return ""
	}
	return strings.SplitN(rel, "/", 2)[0]
}

// layerOf 返回包在域内的层：api / application / domain / infrastructure / interfaces。
func layerOf(importPath string) string {
	rel := strings.TrimPrefix(importPath, moduleBase+"/")
	for _, seg := range strings.Split(rel, "/") {
		switch seg {
		case "api", "application", "domain", "infrastructure", "interfaces":
			return seg
		}
	}
	return ""
}

// endOf 返回包所属端名（仅当路径形如 <domain>/interfaces/<end>），否则 ""。
func endOf(importPath string) string {
	rel := strings.TrimPrefix(importPath, moduleBase+"/")
	segs := strings.Split(rel, "/")
	if len(segs) < 3 || segs[1] != "interfaces" {
		return ""
	}
	for _, e := range ends {
		if segs[2] == e {
			return e
		}
	}
	return ""
}

func isDomain(importPath string) bool {
	for _, d := range domains {
		if topOf(importPath) == d {
			return true
		}
	}
	return false
}

// --- LayerRulesTest: layered_dependencies_are_respected ---

func TestLayeredDependencies(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		for _, imp := range p.Imports {
			if !isDomain(imp) {
				continue
			}
			impLayer, impDomain := layerOf(imp), topOf(imp)
			srcLayer, srcDomain := layerOf(path), topOf(path)

			switch impLayer {
			case "interfaces":
				// interfaces 不得被任何层访问；只有组合根（app）与同域 interfaces 自己可以碰
				allowed := srcDomain == impDomain && srcLayer == "interfaces" || topOf(path) == "app"
				if !allowed {
					t.Errorf("%s 访问了 %s 层包 %s（interfaces 不得被任何实现层访问）", path, impLayer, imp)
				}
			case "application":
				// application 只能被 interfaces 访问（+组合根）
				allowed := srcDomain == impDomain && srcLayer == "interfaces" || topOf(path) == "app"
				if !allowed {
					t.Errorf("%s 访问了 %s 域的 application 包 %s（只允许 interfaces 与组合根）", path, impDomain, imp)
				}
			case "domain":
				// domain 只能被 application / infrastructure / 同域 domain 访问
				allowed := srcDomain == impDomain &&
					(srcLayer == "domain" || srcLayer == "application" || srcLayer == "infrastructure")
				if !allowed {
					t.Errorf("%s 访问了 %s 域的 domain 包 %s（只允许 application/infrastructure/同域 domain）", path, impDomain, imp)
				}
			case "infrastructure":
				// infrastructure 只能被 application 访问（+组合根装配）
				allowed := srcDomain == impDomain && srcLayer == "application" || topOf(path) == "app"
				if !allowed {
					t.Errorf("%s 访问了 %s 域的 infrastructure 包 %s（只允许 application 与组合根）", path, impDomain, imp)
				}
			}
		}
	}
}

// --- LayerRulesTest: api_packages_do_not_depend_on_implementation_layers ---

func TestApiPackagesDoNotDependOnImplementationLayers(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		if layerOf(path) != "api" {
			continue
		}
		for _, imp := range p.Imports {
			switch layerOf(imp) {
			case "application", "domain", "infrastructure", "interfaces":
				t.Errorf("契约包 %s 不得依赖实现层（%s）：%s", path, layerOf(imp), imp)
			}
		}
	}
}

// --- ApiContractRulesTest: api_interfaces_are_implemented_only_by_providers ---
//
// Java 用反射扫描 getAllRawInterfaces；Go 用 go/types 类型检查器：
// 遍历模块内全部具名类型，凡是实现了 <domain>/api 包里 XxxApi 接口的，
// 其所在包必须位于 interfaces/provider —— 防止结构化类型让应用服务
// "无意中实现"契约（Go 特有的风险，Java 名义类型系统没有这个问题）。
func TestApiInterfacesAreImplementedOnlyByProviders(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedDeps | packages.NeedImports,
		Dir:  "../..",
	}
	pkgs, err := packages.Load(cfg, "./...")
	require.NoError(t, err)
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("load %s: %v", p.PkgPath, e)
		}
	}

	// 1. 收集契约接口：<domain>/api 包内、名字以 Api 结口的接口类型
	type apiIface struct {
		name  string
		iface *types.Interface
	}
	var ifaces []apiIface
	for _, p := range pkgs {
		if !strings.HasSuffix(p.PkgPath, "/api") || !strings.HasPrefix(p.PkgPath, moduleBase) {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			iface, ok := tn.Type().Underlying().(*types.Interface)
			if !ok {
				continue
			}
			ifaces = append(ifaces, apiIface{name: p.PkgPath + "." + name, iface: iface})
		}
	}
	require.Len(t, ifaces, 3, "应有 system/grade/attendance 三个契约接口")

	// 2. 全模块具名类型逐一检查
	for _, p := range pkgs {
		if !strings.HasPrefix(p.PkgPath, moduleBase) {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			if _, isIface := tn.Type().Underlying().(*types.Interface); isIface {
				continue
			}
			for _, ai := range ifaces {
				ptr := types.NewPointer(tn.Type())
				if types.Implements(tn.Type(), ai.iface) || types.Implements(ptr, ai.iface) {
					if !strings.Contains(p.PkgPath, "/interfaces/provider") {
						t.Errorf("%s.%s 实现了契约接口 %s（实现者必须位于 interfaces/provider）",
							p.PkgPath, name, ai.name)
					}
				}
			}
		}
	}
}

// --- DomainIsolationRulesTest: cross_domain_dependencies_use_api_package_only ---

func TestCrossDomainDependenciesUseApiPackageOnly(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		if !isDomain(path) {
			continue
		}
		for _, imp := range p.Imports {
			if !isDomain(imp) {
				continue
			}
			if topOf(path) == topOf(imp) {
				continue // 同域内部依赖
			}
			if layerOf(imp) != "api" {
				t.Errorf("%s（%s 域）跨域依赖了 %s 域的 %s —— 只允许走对方 api 契约包",
					path, topOf(path), topOf(imp), imp)
			}
		}
	}
	// domains_are_free_of_cycles：Go 编译器禁止 import 环，无需检测（对应 Java slices.beFreeOfCycles）。
}

// --- DomainPurityRulesTest: domain_does_not_depend_on_frameworks / other layers ---

func TestDomainPurity(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		if layerOf(path) != "domain" || !isDomain(path) {
			continue
		}
		for _, imp := range p.Imports {
			// 框架禁令（对应 spring/jakarta/javax/fasterxml）
			if strings.HasPrefix(imp, "github.com/gin-gonic/gin") || strings.HasPrefix(imp, "gorm.io/") {
				t.Errorf("domain 包 %s 不得依赖框架：%s", path, imp)
			}
			// 其他实现层禁令
			switch layerOf(imp) {
			case "application", "infrastructure", "interfaces":
				t.Errorf("domain 包 %s 不得依赖 %s 层：%s", path, layerOf(imp), imp)
			}
			// 第三方禁令：domain 只允许标准库与模块内包（对应 Java 只允许 java..）
			if !isDomain(imp) && imp != moduleBase+"/shared" && !strings.HasPrefix(imp, moduleBase+"/shared/") {
				first := imp
				if i := strings.Index(imp, "/"); i >= 0 {
					first = imp[:i]
				}
				if strings.Contains(first, ".") {
					t.Errorf("domain 包 %s 不得依赖第三方库：%s", path, imp)
				}
			}
		}
	}
}

// --- EndIsolationRulesTest: ends_do_not_depend_on_each_other ---

func TestEndsDoNotDependOnEachOther(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		srcEnd := endOf(path)
		if srcEnd == "" {
			continue
		}
		for _, imp := range p.Imports {
			impEnd := endOf(imp)
			if impEnd == "" || impEnd == srcEnd {
				continue
			}
			t.Errorf("端 %s（%s）不得依赖其他端 %s：%s", srcEnd, path, impEnd, imp)
		}
	}
}

// --- PlatformRulesTest: platform_does_not_depend_on_business_domains ---

func TestPlatformDoesNotDependOnBusinessDomains(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		if topOf(path) != "platform" {
			continue
		}
		for _, imp := range p.Imports {
			if isDomain(imp) {
				t.Errorf("平台层包 %s 不得依赖业务域：%s", path, imp)
			}
		}
	}
}

// --- PlatformRulesTest: shared_kernel_is_dependency_free ---

func TestSharedKernelPurity(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		if topOf(path) != "shared" {
			continue
		}
		for _, imp := range p.Imports {
			if isDomain(imp) || topOf(imp) == "platform" {
				t.Errorf("共享内核包 %s 不得依赖业务域或平台层：%s", path, imp)
			}
			// Java 版只允许 java..（JDK 标准库）；Go 标准库没有 UUID，
			// google/uuid 是事实标准等价物 —— 唯一豁免，其余第三方一律禁止。
			if imp != "github.com/google/uuid" && !isDomain(imp) && topOf(imp) == "" {
				first := imp
				if i := strings.Index(imp, "/"); i >= 0 {
					first = imp[:i]
				}
				if strings.Contains(first, ".") {
					t.Errorf("共享内核包 %s 只允许标准库与 uuid，不得依赖：%s", path, imp)
				}
			}
		}
	}
}

// --- PlatformRulesTest: system_domain_is_a_pure_provider ---

func TestSystemDomainIsPureProvider(t *testing.T) {
	pkgs := listModulePackages(t)
	for path, p := range pkgs {
		if topOf(path) != "system" {
			continue
		}
		for _, imp := range p.Imports {
			if d := topOf(imp); d == "grade" || d == "attendance" {
				t.Errorf("system 域是纯提供方，不得依赖其他域：%s", imp)
			}
		}
	}
}
