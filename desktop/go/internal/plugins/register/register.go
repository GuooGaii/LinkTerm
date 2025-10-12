package register

//go:generate go run ./generate

// Register ensures all statically linked plugins execute其 init 逻辑（自动生成的空白导入会触发注册）.
func Register() {}
