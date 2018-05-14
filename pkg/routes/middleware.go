package routes

import (
	"net/textproto"

	"git.containerum.net/ch/resource-service/pkg/rsErrors"
	"github.com/containerum/cherry"
	"github.com/containerum/utils/httputil"
	"github.com/gin-gonic/gin"
	"gopkg.in/go-playground/validator.v9"
)

func (tv *TranslateValidate) HandleError(err error) (int, *cherry.Err) {
	switch err.(type) {
	case *cherry.Err:
		e := err.(*cherry.Err)
		return e.StatusHTTP, e
	default:
		return rserrors.ErrInternal().StatusHTTP, rserrors.ErrInternal().AddDetailsErr(err)
	}
}

func (tv *TranslateValidate) BadRequest(ctx *gin.Context, err error) (int, *cherry.Err) {
	if validationErr, ok := err.(validator.ValidationErrors); ok {
		ret := rserrors.ErrValidation()
		for _, fieldErr := range validationErr {
			if fieldErr == nil {
				continue
			}
			t, _ := tv.FindTranslator(httputil.GetAcceptedLanguages(ctx.Request.Context())...)
			ret.AddDetailF("Field %s: %s", fieldErr.Namespace(), fieldErr.Translate(t))
		}
		return ret.StatusHTTP, ret
	}
	return rserrors.ErrValidation().StatusHTTP, rserrors.ErrValidation().AddDetailsErr(err)
}

func (tv *TranslateValidate) ValidateHeaders(headerTagMap map[string]string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		headerErr := make(map[string]validator.ValidationErrors)
		for header, tag := range headerTagMap {
			ferr := tv.VarCtx(ctx.Request.Context(), ctx.GetHeader(textproto.CanonicalMIMEHeaderKey(header)), tag)
			if ferr != nil {
				headerErr[header] = ferr.(validator.ValidationErrors)
			}
		}
		if len(headerErr) > 0 {
			ret := rserrors.ErrValidation()
			for header, fieldErrs := range headerErr {
				for _, fieldErr := range fieldErrs {
					if fieldErr == nil {
						continue
					}
					t, _ := tv.FindTranslator(httputil.GetAcceptedLanguages(ctx.Request.Context())...)
					ret.AddDetailF("Header %s: %s", header, fieldErr.Translate(t))
				}
			}
			ctx.AbortWithStatusJSON(ret.StatusHTTP, ret)
			return
		}
	}
}
