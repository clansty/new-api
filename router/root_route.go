package router

import "github.com/gin-gonic/gin"

func registerRootRoute(group *gin.RouterGroup, method string, handlers ...gin.HandlerFunc) {
	group.Handle(method, "", handlers...)
	group.Handle(method, "/", handlers...)
}
