package structure

// Point struct：Go 没有 class 关键字，struct + 方法 = 数据 + 行为。
type Point struct {
	X, Y int
}

// ShiftValue 值接收者：拿到副本，改不动原对象（适合纯函数式的只读方法）。
func (p Point) ShiftValue(delta int) Point {
	p.X += delta // 只改副本
	return p
}

// Shift 指针接收者：能改原对象，也避免大结构体的拷贝 —— 实际工程里最常用。
func (p *Point) Shift(delta int) {
	p.X += delta
}

// Move 演示两种接收者的差异。
// 规则：值类型的变量调用指针方法时，编译器自动取址（p.Shift 等价于 (&p).Shift）。
func Move() (valueReceiverX, pointerReceiverX int) {
	p := Point{X: 1}
	_ = p.ShiftValue(10) // 副本被改，p 不变
	valueReceiverX = p.X // 仍是 1
	p.Shift(10)          // 原地修改
	pointerReceiverX = p.X
	return
}

// Animal 基础行为（对应 Java 的父类 Animal）。
type Animal struct {
	Name string
}

func (a Animal) Describe() string { return "我是 " + a.Name }

// Dog 嵌入代替继承：
//   - 没有 extends，嵌入（匿名字段）自动"提升"字段与方法；
//   - 子类型方法遮蔽（对应 @Override）；
//   - 没有 super —— 用显式的嵌入字段名调用被遮蔽的版本。
type Dog struct {
	Animal // 嵌入
	Breed  string
}

func (d Dog) Describe() string { // 遮蔽 Animal.Describe
	return d.Animal.Describe() + "，是只 " + d.Breed // 显式调"父类"版本
}

// NewDog 构造惯例：没有构造函数，工厂函数 NewXxx 即构造器（对应 Java 构造器/静态工厂）。
func NewDog(name, breed string) *Dog {
	return &Dog{Animal: Animal{Name: name}, Breed: breed}
}

// UserDTO struct tag：JSON/ORM 的元数据挂在字段上（对应 Java 注解 @JsonProperty）。
// P4~P6 架构篇里跨服务契约的 json tag 就是这个机制。
type UserDTO struct {
	Name string `json:"name"`
	Age  int    `json:"age,omitempty"` // omitempty：零值不序列化
}
