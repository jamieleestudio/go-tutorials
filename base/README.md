# base —— 语法基础

对应 java-tutorials 的 [`base`](../../../java-tutorials/base)（核心是 `base/se`：foundation / structure / concurrent / algorithm / ocp / utils）。
面向 **Java 开发者学 Go**：每个主题文件把 Java 的同名概念逐条标注成 Go 写法，**测试即文档** —— 断言就是预期语义，跑 `go test` 即可验证所有结论。

## 模块映射

| Go 包 | Java 对应 | 内容 |
|---|---|---|
| `foundation` | `se/foundation`（EnumTest 等） | 变量/类型/零值、iota 枚举、控制流（if/for/switch/label）、函数（多返回值/闭包/defer）、错误处理（哨兵错误/包装/errors.Is/As/panic-recover） |
| `structure` | `se/structure` + Java 集合/OOP | 数组 vs 切片、切片陷阱（写穿/三索引）、map、struct 与方法接收者、嵌入代替继承、隐式接口、泛型、Java 嵌套类的四种对应物 |
| `concurrent` | `se/concurrent`（ReentrantLockTest） | goroutine + WaitGroup、channel 全家桶（无缓冲/select/range/流水线/单向/worker pool）、Mutex/TryLock/RWMutex/Once、context（取消/超时/传值） |
| `algorithm` | `se/algorithm`（同名同题） | QuickSort 三种 partition、NumberSort 树形菜单排序（14 节点 DFS）、ListStack 队列轮转实现栈、FrogJump 跳台阶（递归/记忆化/滚动变量） |
| `utils` | `se/utils`（DateUtils 系列） | time 布局/时区坑/AddDate、fmt 动词、strconv、strings、regexp |

（Java 版的 `se/ocp` 是 OCP 认证考点集，与 Go 语言无关，不迁移。）

## Java ↔ Go 语法对照速查

| Java | Go |
|---|---|
| `int x = 1;` / `var x = 1;`(10+) | `x := 1`（短声明）或 `var x int` |
| `enum Weekday { ... }` | `type Weekday int` + `iota` 常量块 |
| `char`（UTF-16 单元） | `rune`（Unicode 码点，UTF-8 解码） |
| `String[] args` / `int[] arr` | `[]string` / `[]int`（切片，len/cap 动态） |
| `ArrayList<T>` / `HashMap<K,V>` | `[]T` / `map[K]V` |
| 异常 `throw / try-catch-finally` | 返回值 `error` + `defer`；`panic/recover` 仅留给程序 bug |
| `extends` / `implements` | struct 嵌入 + 接口【隐式实现】 |
| lambda `x -> x * 2` | 闭包 `func(x int) int { return x * 2 }` |
| `Thread` / 线程池 | `go func()` / worker pool（channel 当队列） |
| `synchronized` / `ReentrantLock` | `sync.Mutex`（含 `TryLock`） |
| `CountDownLatch` | `sync.WaitGroup` |
| `AtomicLong` | `atomic.Int64` |
| `Future.get(timeout)` | `context.WithTimeout` + `select` |
| `String.format` | `fmt.Sprintf`（`%v` 万能动词） |
| `SimpleDateFormat("yyyy-MM-dd")` | `time.Format("2006-01-02")`（参考时间） |
| 嵌套类/匿名类 | 包级 struct / struct 嵌入 / 函数内 type / 闭包适配器 |

## 运行

```bash
# workspace 已含 ./base，在仓库根目录：
go test ./base/...            # 全部主题
go test -race ./base/concurrent   # 并发主题加 -race 抓数据竞争
go test ./base/algorithm -v       # 算法主题看每个断言
```

推荐阅读顺序：`foundation` → `structure` → `concurrent` → `algorithm` → `utils`，
每个文件顶部注释都有 Java 对照说明，配对阅读 `_test.go` 里的断言。
