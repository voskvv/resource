package handlers

import (
	"net/http"

	m "git.containerum.net/ch/resource-service/pkg/router/middleware"
	"git.containerum.net/ch/resource-service/pkg/server"
	"github.com/gin-gonic/gin"
)

type ResourceHandlers struct {
	server.ResourcesActions
	*m.TranslateValidate
}

// swagger:operation GET /resources Resources GetResourcesCountHandler
// Get resources count.
//
// ---
// x-method-visibility: public
// parameters:
//  - name: namespace
//    in: path
//    type: string
//    required: true
// responses:
//  '200':
//    description: resources count
//    schema:
//      $ref: '#/definitions/GetResourcesCountResponse'
//  default:
//    $ref: '#/responses/error'
func (h *ResourceHandlers) GetResourcesCountHandler(ctx *gin.Context) {
	resp, err := h.GetResourcesCount(ctx.Request.Context())
	if err != nil {
		ctx.AbortWithStatusJSON(h.HandleError(err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// swagger:operation DELETE /namespaces/{namespace} Resources DeleteAllResourcesInNamespaceHandler
// Delete all resources in namespace.
//
// ---
// x-method-visibility: private
// parameters:
//  - name: namespace
//    in: path
//    type: string
//    required: true
// responses:
//  '202':
//    description: all resources in namespace deleted
//  default:
//    $ref: '#/responses/error'
func (h *ResourceHandlers) DeleteAllResourcesInNamespaceHandler(ctx *gin.Context) {
	if err := h.DeleteAllResourcesInNamespace(ctx.Request.Context(), ctx.Param("namespace")); err != nil {
		ctx.AbortWithStatusJSON(h.HandleError(err))
		return
	}

	ctx.Status(http.StatusAccepted)
}

// swagger:operation DELETE /namespaces Resources DeleteAllResourcesHandler
// Delete all user resources.
//
// ---
// x-method-visibility: private
// parameters:
//  - $ref: '#/parameters/UserIDHeader'
//  - $ref: '#/parameters/UserRoleHeader'
// responses:
//  '202':
//    description: all user resources deleted
//  default:
//    $ref: '#/responses/error'
func (h *ResourceHandlers) DeleteAllResourcesHandler(ctx *gin.Context) {
	if err := h.DeleteAllUserResources(ctx.Request.Context()); err != nil {
		ctx.AbortWithStatusJSON(h.HandleError(err))
		return
	}

	ctx.Status(http.StatusAccepted)
}
