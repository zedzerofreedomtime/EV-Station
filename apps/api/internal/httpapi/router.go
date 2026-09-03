package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rbc/ev-station/apps/api/internal/config"
)

func NewRouter(cfg config.Config, handler *Handler) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		MaxAge:       12 * time.Hour,
	}))
	router.GET("/health", handler.Health)
	api := router.Group("/api/v1")
	api.POST("/sites", handler.CreateSite)
	api.GET("/sites", handler.ListSites)
	api.POST("/maps/resolve", handler.ResolveGoogleMapsURL)
	api.GET("/sites/:id/latest-analysis", handler.GetLatestCompletedAnalysisForSite)
	api.GET("/sites/:id", handler.GetSite)
	api.PUT("/sites/:id", handler.UpdateSite)
	api.DELETE("/sites/:id", handler.DeleteSite)
	api.POST("/sites/:id/analyses", handler.RunAnalysis)
	api.GET("/analyses/:id", handler.GetAnalysis)
	api.POST("/analyses/:id/recalculate-preliminary", handler.RecalculatePreliminary)
	api.POST("/analyses/:id/ai-assessment", handler.GenerateAIAssessment)
	api.GET("/geocoding/search", handler.SearchAddress)
	api.GET("/data-sources", handler.GetDataSources)
	api.POST("/financial/calculate", handler.CalculateFinancial)
	api.GET("/financial/plans", handler.GetFranchisePlans)
	api.GET("/financial/plans/:code", handler.GetFranchisePlan)
	api.GET("/scoring/config", handler.GetScoringConfig)
	return router
}
