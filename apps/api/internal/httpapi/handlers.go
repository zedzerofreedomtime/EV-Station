package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/analysis"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/financial"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
	"github.com/rbc/ev-station/apps/api/internal/site"
)

type Handler struct {
	sites    *site.Service
	analyses *analysis.Service
	weights  map[string]float64
	geocoder provider.Geocoder
}

func NewHandler(sites *site.Service, analyses *analysis.Service, geocoder provider.Geocoder, weights map[string]float64) *Handler {
	return &Handler{sites: sites, analyses: analyses, geocoder: geocoder, weights: weights}
}

func (h *Handler) SearchAddress(c *gin.Context) {
	query := c.Query("q")
	results, err := h.geocoder.Search(c.Request.Context(), query, parseLimit(c.Query("limit"), 5))
	if errors.Is(err, provider.ErrInvalidGeocodingQuery) {
		writeError(c, http.StatusBadRequest, "INVALID_GEOCODING_QUERY", err.Error())
		return
	}
	if err != nil {
		writeError(c, http.StatusBadGateway, "GEOCODING_UNAVAILABLE", "The free geocoding provider is temporarily unavailable.")
		return
	}
	if results == nil {
		results = []provider.GeocodingResult{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func (h *Handler) GetDataSources(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": provider.DataSourceCatalog()})
}

func (h *Handler) Health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }

func (h *Handler) CreateSite(c *gin.Context) {
	var input domain.CreateSiteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.sites.Create(c.Request.Context(), input)
	if errors.Is(err, site.ErrInvalidLocation) {
		writeError(c, http.StatusBadRequest, "INVALID_LOCATION", err.Error())
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_CREATE_FAILED", "Unable to save the site.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *Handler) ListSites(c *gin.Context) {
	result, err := h.sites.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_LIST_FAILED", "Unable to load sites.")
		return
	}
	if result == nil {
		result = []domain.Site{}
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) GetSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := h.sites.Get(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_LOAD_FAILED", "Unable to load the site.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) RunAnalysis(c *gin.Context) {
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		RadiusMeters int `json:"radiusMeters"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
	}
	if body.RadiusMeters != 0 && body.RadiusMeters != 1000 && body.RadiusMeters != 3000 && body.RadiusMeters != 5000 {
		writeError(c, http.StatusBadRequest, "INVALID_RADIUS", "Radius must be 1000, 3000, or 5000 meters.")
		return
	}
	result, err := h.analyses.Run(c.Request.Context(), siteID, body.RadiusMeters)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ANALYSIS_FAILED", "Unable to complete the analysis.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *Handler) GetAnalysis(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := h.analyses.Get(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "Analysis not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ANALYSIS_LOAD_FAILED", "Unable to load the analysis.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) CalculateFinancial(c *gin.Context) {
	var input financial.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_FINANCIAL_INPUT", err.Error())
		return
	}
	result, err := financial.Calculate(input)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_FINANCIAL_INPUT", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) GetFranchisePlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": financial.FranchisePlans()})
}

func (h *Handler) GetFranchisePlan(c *gin.Context) {
	plan, err := financial.GetFranchisePlan(c.Param("code"))
	if errors.Is(err, financial.ErrUnknownPlan) {
		writeError(c, http.StatusNotFound, "FRANCHISE_PLAN_NOT_FOUND", "Franchise plan not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plan})
}

func (h *Handler) GetScoringConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"weights": h.weights, "status": "provisional", "total": 1}})
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_ID", "The supplied id is invalid.")
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func parseLimit(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

var _ = scoring.DefaultWeights
