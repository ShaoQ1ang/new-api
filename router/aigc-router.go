package router

import (
	aigchandler "github.com/QuantumNous/new-api/aigc/handler"
	aigcrepository "github.com/QuantumNous/new-api/aigc/repository"
	aigcrouter "github.com/QuantumNous/new-api/aigc/router"
	aigcservice "github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func SetAigcRouter(engine *gin.Engine) {
	profiles := aigcrepository.New(model.DB)
	catalog := aigcservice.NewCatalogService(profiles, aigcservice.NewModelAvailability())
	modelHandler := aigchandler.NewModelHandler(catalog)
	admin := aigcservice.NewAdminService(profiles)
	adminHandler := aigchandler.NewAdminModelHandler(admin, catalog)
	upstreamModels := aigcservice.NewUpstreamModelService(aigcservice.NewModelUpstreamSource())
	upstreamHandler := aigchandler.NewUpstreamModelHandler(upstreamModels)
	profileImporter := aigcservice.NewProfileImportService(profiles, upstreamModels)
	importHandler := aigchandler.NewProfileImportHandler(profileImporter)
	aigcrouter.RegisterRelayRoutes(engine, modelHandler)
	aigcrouter.RegisterAPIRoutes(engine, adminHandler, upstreamHandler, importHandler)
}
