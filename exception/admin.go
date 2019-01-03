package exception

var (
	AdminExist    = New("管理员已存在")
	AdminNotExist = New("管理员不存在")
	AdminNotSuper = New("只有超级管理员才能操作")
)
