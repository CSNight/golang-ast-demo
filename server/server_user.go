// go:controller(path="/users",name="users")
package server

import (
	"github.com/gofiber/fiber/v2"
)

// go:interface(method="GET",path="/all",auth="USER_QUERY",opLog="用户目录")
func (srv *AdminServer) GetAllUser(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="GET",path="/query",auth="USER_QUERY",opLog="用户搜索")
func (srv *AdminServer) QueryUsers(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="POST",path="/user/add",auth="USER_ADD",opLog="新增用户")
func (srv *AdminServer) NewUser(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="PUT",path="/user/edit",auth="USER_INFO_EDIT",opLog="修改用户信息")
func (srv *AdminServer) UpdateUserInfo(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="PUT",path="/pwd/edit",auth="USER_UPDATE",opLog="修改用户密码")
func (srv *AdminServer) UpdateUserPass(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="POST",path="/avatar/edit",auth="USER_INFO_EDIT",opLog="修改用户头像")
func (srv *AdminServer) ChangeAvatar(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="DELETE",path="/user/id/:id",auth="USER_DEL",opLog="通过ID删除用户")
func (srv *AdminServer) DeleteUserById(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="DELETE",path="/user/name/:name",auth="USER_DEL",opLog="通过用户名删除用户")
func (srv *AdminServer) DeleteUserByName(ctx *fiber.Ctx) error {
	return nil
}
