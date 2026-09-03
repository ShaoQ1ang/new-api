package router

import (
	aigcexecution "github.com/QuantumNous/new-api/aigc/execution"
	aigchandler "github.com/QuantumNous/new-api/aigc/handler"
	aigcrepository "github.com/QuantumNous/new-api/aigc/repository"
	aigcrouter "github.com/QuantumNous/new-api/aigc/router"
	aigcservice "github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type aigcHandlers struct {
	models        *aigchandler.ModelHandler
	generations   *aigchandler.GenerationHandler
	pricing       *aigchandler.PricingHandler
	admin         *aigchandler.AdminModelHandler
	upstream      *aigchandler.UpstreamModelHandler
	profileImport *aigchandler.ProfileImportHandler
}

func buildAigcHandlers(db *gorm.DB) aigcHandlers {
	profiles := aigcrepository.New(db)
	availability := aigcservice.NewModelAvailability()
	catalog := aigcservice.NewCatalogService(profiles, availability)
	modelHandler := aigchandler.NewModelHandler(catalog)
	admin := aigcservice.NewAdminService(profiles)
	adminHandler := aigchandler.NewAdminModelHandler(admin, catalog)
	upstreamModels := aigcservice.NewUpstreamModelService(aigcservice.NewModelUpstreamSource())
	upstreamHandler := aigchandler.NewUpstreamModelHandler(upstreamModels)
	pricing := aigcservice.NewPricingService(profiles, availability, aigcservice.NewModelUpstreamSource())
	pricingHandler := aigchandler.NewPricingHandler(pricing)
	profileImporter := aigcservice.NewProfileImportService(profiles, upstreamModels)
	importHandler := aigchandler.NewProfileImportHandler(profileImporter)

	resolver := aigcservice.NewGenerationResolver(profiles, availability)
	idempotency := aigcservice.NewIdempotencyService(profiles, nil)
	taskWorkflow := &relay.TaskWorkflow{OnChannelError: controller.ProcessChannelError}
	taskExecutor := aigcexecution.NewTaskExecutor(taskWorkflow, aigcexecution.ModelTaskStore{})
	syncWorkflow := &relay.SyncWorkflow{OnChannelError: controller.ProcessChannelError}
	syncExecutor := aigcexecution.NewSyncExecutor(syncWorkflow)
	executor := aigcexecution.NewCompositeExecutor(syncExecutor, taskExecutor)
	generations := aigcservice.NewGenerationService(resolver, idempotency, profiles, executor)
	generationHandler := aigchandler.NewGenerationHandler(generations)

	return aigcHandlers{
		models: modelHandler, generations: generationHandler, pricing: pricingHandler, admin: adminHandler,
		upstream: upstreamHandler, profileImport: importHandler,
	}
}

func SetAigcRouter(engine *gin.Engine) {
	handlers := buildAigcHandlers(model.DB)
	aigcrouter.RegisterRelayRoutes(engine, handlers.models, handlers.generations)
	aigcrouter.RegisterPricingRoute(engine, handlers.pricing)
	aigcrouter.RegisterAPIRoutes(engine, handlers.admin, handlers.upstream, handlers.profileImport)
}
