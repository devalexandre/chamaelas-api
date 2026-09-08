package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"chamaelas-api/internal/models"
)

func (m *Module) ListPricing(c echo.Context) error {
	ctx := c.Request().Context()

	rules, err := m.pricing.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	cities, err := m.cities.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	categories, err := m.categories.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	cityByID := map[string]string{}
	for _, city := range cities {
		cityByID[city.ID] = city.Name + "/" + city.UF
	}
	categoryByID := map[string]string{}
	for _, category := range categories {
		categoryByID[category.ID] = category.Name
	}

	for i := range rules {
		if rules[i].CityID != nil {
			rules[i].CityName = cityByID[*rules[i].CityID]
		} else {
			rules[i].CityName = "Todas as cidades"
		}
		if rules[i].CategoryID != nil {
			rules[i].CategoryName = categoryByID[*rules[i].CategoryID]
		} else {
			rules[i].CategoryName = "Todas as categorias"
		}
	}

	return render(c, "pricing", "pricing.html", map[string]any{
		"Rules":      rules,
		"Cities":     cities,
		"Categories": categories,
		"Weekdays":   models.Weekdays,
	})
}

// valueAt returns slice[i] or "" if i is out of range — form arrays from
// different <input> groups aren't guaranteed to line up in length.
func valueAt(slice []string, i int) string {
	if i < 0 || i >= len(slice) {
		return ""
	}
	return slice[i]
}

// parseRuleForm reads the fields shared by the create and edit forms —
// scope (city/category), schedule (day/time window), and the dynamic list
// of km brackets — used by both CreatePricingRule and UpdatePricingRule.
func parseRuleForm(c echo.Context) (dayOfWeek int, startTime, endTime string, cityID, categoryID *string, brackets []models.PricingBracket, ok bool) {
	var err error
	dayOfWeek, err = strconv.Atoi(c.FormValue("dayOfWeek"))
	if err != nil {
		return
	}
	startTime = c.FormValue("startTime")
	endTime = c.FormValue("endTime")
	if startTime == "" || endTime == "" {
		return
	}

	if v := c.FormValue("cityId"); v != "" {
		cityID = &v
	}
	if v := c.FormValue("categoryId"); v != "" {
		categoryID = &v
	}

	params, err := c.FormParams()
	if err != nil {
		return
	}
	kmFroms := params["kmFrom[]"]
	kmTos := params["kmTo[]"]
	prices := params["price[]"]
	pricesPerKm := params["pricePerKm[]"]

	brackets = make([]models.PricingBracket, 0, len(kmFroms))
	for i := range kmFroms {
		kmFrom, err := strconv.ParseFloat(kmFroms[i], 64)
		if err != nil {
			continue
		}
		// Price and PricePerKm are each optional (one can carry a flat fee,
		// the other a per-km rate, or both together) — only skip the row if
		// neither was filled in at all.
		price, priceErr := strconv.ParseFloat(valueAt(prices, i), 64)
		pricePerKm, perKmErr := strconv.ParseFloat(valueAt(pricesPerKm, i), 64)
		if priceErr != nil && perKmErr != nil {
			continue
		}
		var kmTo *float64
		if i < len(kmTos) && kmTos[i] != "" {
			if v, err := strconv.ParseFloat(kmTos[i], 64); err == nil {
				kmTo = &v
			}
		}
		brackets = append(brackets, models.PricingBracket{KmFrom: kmFrom, KmTo: kmTo, Price: price, PricePerKm: pricePerKm})
	}
	if len(brackets) == 0 {
		return
	}

	ok = true
	return
}

func (m *Module) CreatePricingRule(c echo.Context) error {
	dayOfWeek, startTime, endTime, cityID, categoryID, brackets, ok := parseRuleForm(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/admin/pricing")
	}

	rule := &models.PricingRule{
		ID:         uuid.NewString(),
		CityID:     cityID,
		CategoryID: categoryID,
		DayOfWeek:  dayOfWeek,
		StartTime:  startTime,
		EndTime:    endTime,
		CreatedAt:  time.Now().UTC(),
	}
	if err := m.pricing.Create(c.Request().Context(), rule, brackets); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/pricing")
}

func (m *Module) EditPricingRulePage(c echo.Context) error {
	ctx := c.Request().Context()

	rule, err := m.pricing.FindByID(ctx, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "regra não encontrada")
	}

	cities, err := m.cities.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	categories, err := m.categories.ListAll(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	selectedCityID := ""
	if rule.CityID != nil {
		selectedCityID = *rule.CityID
	}
	selectedCategoryID := ""
	if rule.CategoryID != nil {
		selectedCategoryID = *rule.CategoryID
	}

	return render(c, "pricing", "pricing_edit.html", map[string]any{
		"Rule":               rule,
		"Cities":             cities,
		"Categories":         categories,
		"Weekdays":           models.Weekdays,
		"SelectedCityID":     selectedCityID,
		"SelectedCategoryID": selectedCategoryID,
	})
}

func (m *Module) UpdatePricingRule(c echo.Context) error {
	dayOfWeek, startTime, endTime, cityID, categoryID, brackets, ok := parseRuleForm(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/admin/pricing/"+c.Param("id")+"/edit")
	}

	rule := &models.PricingRule{
		ID:         c.Param("id"),
		CityID:     cityID,
		CategoryID: categoryID,
		DayOfWeek:  dayOfWeek,
		StartTime:  startTime,
		EndTime:    endTime,
	}
	if err := m.pricing.Update(c.Request().Context(), rule, brackets); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/pricing")
}

func (m *Module) ClonePricingRule(c echo.Context) error {
	if err := m.pricing.CloneToAllDays(c.Request().Context(), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/pricing")
}

func (m *Module) DeletePricingRule(c echo.Context) error {
	if err := m.pricing.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/pricing")
}
