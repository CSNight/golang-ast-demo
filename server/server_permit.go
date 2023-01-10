// go:controller(path="/permits",name="permits")
package server

import (
	"github.com/gofiber/fiber/v2"
)

// go:interface(method="GET",path="/all",auth="RIGHTS_QUERY",opLog="查询权限列表")
func (srv *AdminServer) GetPermissions(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="GET",path="/query",auth="RIGHTS_QUERY",opLog="搜索权限")
func (srv *AdminServer) QueryPermissions(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="POST",path="/permit/add",auth="RIGHTS_ADD",opLog="创建权限")
func (srv *AdminServer) CreatePermission(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="PUT",path="/permit/edit",auth="RIGHTS_UPDATE",opLog="修改权限")
func (srv *AdminServer) UpdatePermission(ctx *fiber.Ctx) error {
	return nil
}

// go:interface(method="DELETE",path="/permit/del/:id",auth="RIGHTS_DEL",opLog="删除权限")
func (srv *AdminServer) DeletePermission(ctx *fiber.Ctx) error {
	return nil
}
