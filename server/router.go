package server

import "github.com/gofiber/fiber/v2"

func (srv *AdminServer) permitsRegister(root fiber.Router) {
	root.Get("/all", srv.GetPermissions)
	root.Get("/query", srv.QueryPermissions)
	root.Post("/permit/add", srv.CreatePermission)
	root.Put("/permit/edit", srv.UpdatePermission)
	root.Delete("/permit/del/:id", srv.DeletePermission)
}
func (srv *AdminServer) usersRegister(root fiber.Router) {
	root.Get("/all", srv.GetAllUser)
	root.Get("/query", srv.QueryUsers)
	root.Post("/user/add", srv.NewUser)
	root.Put("/user/edit", srv.UpdateUserInfo)
	root.Put("/pwd/edit", srv.UpdateUserPass)
	root.Post("/avatar/edit", srv.ChangeAvatar)
	root.Delete("/user/id/:id", srv.DeleteUserById)
	root.Delete("/user/name/:name", srv.DeleteUserByName)
}
func (srv *AdminServer) Register(root fiber.Router) {
	permits := root.Group("/permits")
	users := root.Group("/users")
	srv.permitsRegister(permits)
	srv.usersRegister(users)
}
