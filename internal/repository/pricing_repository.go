package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vingarcia/ksql"

	"chamaelas-api/internal/config"
	"chamaelas-api/internal/database"
	"chamaelas-api/internal/models"
)

var pricingRulesTable = ksql.NewTable("pricing_rules", "id")
var pricingBracketsTable = ksql.NewTable("pricing_brackets", "id")

type PricingRepository struct {
	db  ksql.DB
	cfg config.Config
}

func NewPricingRepository(db ksql.DB, cfg config.Config) *PricingRepository {
	return &PricingRepository{db: db, cfg: cfg}
}

// ListAll returns every rule ordered by day and start time, each with its
// km brackets attached (one extra query per rule — the admin panel only
// ever lists a small number of these, so it's not worth a join here).
func (r *PricingRepository) ListAll(ctx context.Context) ([]models.PricingRule, error) {
	rules := []models.PricingRule{}
	err := r.db.Query(ctx, &rules, "FROM pricing_rules ORDER BY day_of_week, start_time")
	if err != nil {
		return nil, err
	}
	for i := range rules {
		brackets, err := r.bracketsForRule(ctx, rules[i].ID)
		if err != nil {
			return nil, err
		}
		rules[i].Brackets = brackets
	}
	return rules, nil
}

func (r *PricingRepository) FindByID(ctx context.Context, id string) (*models.PricingRule, error) {
	var rule models.PricingRule
	query := fmt.Sprintf("FROM pricing_rules WHERE id = %s", database.Placeholder(r.cfg, 1))
	err := r.db.QueryOne(ctx, &rule, query, id)
	if errors.Is(err, ksql.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	brackets, err := r.bracketsForRule(ctx, rule.ID)
	if err != nil {
		return nil, err
	}
	rule.Brackets = brackets
	return &rule, nil
}

func (r *PricingRepository) bracketsForRule(ctx context.Context, ruleID string) ([]models.PricingBracket, error) {
	brackets := []models.PricingBracket{}
	query := fmt.Sprintf("FROM pricing_brackets WHERE rule_id = %s ORDER BY km_from", database.Placeholder(r.cfg, 1))
	err := r.db.Query(ctx, &brackets, query, ruleID)
	return brackets, err
}

// Create inserts a rule and its km brackets together.
func (r *PricingRepository) Create(ctx context.Context, rule *models.PricingRule, brackets []models.PricingBracket) error {
	return r.db.Transaction(ctx, func(tx ksql.Provider) error {
		if err := tx.Insert(ctx, pricingRulesTable, rule); err != nil {
			return err
		}
		for i := range brackets {
			brackets[i].ID = uuid.NewString()
			brackets[i].RuleID = rule.ID
			if err := tx.Insert(ctx, pricingBracketsTable, &brackets[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// Update replaces a rule's scope/schedule fields and swaps out its km
// brackets entirely for the given set — simpler and safer than diffing
// which brackets changed.
func (r *PricingRepository) Update(ctx context.Context, rule *models.PricingRule, brackets []models.PricingBracket) error {
	return r.db.Transaction(ctx, func(tx ksql.Provider) error {
		updateQuery := fmt.Sprintf(
			"UPDATE pricing_rules SET city_id = %s, category_id = %s, day_of_week = %s, start_time = %s, end_time = %s WHERE id = %s",
			database.Placeholder(r.cfg, 1), database.Placeholder(r.cfg, 2), database.Placeholder(r.cfg, 3),
			database.Placeholder(r.cfg, 4), database.Placeholder(r.cfg, 5), database.Placeholder(r.cfg, 6),
		)
		if _, err := tx.Exec(ctx, updateQuery, rule.CityID, rule.CategoryID, rule.DayOfWeek, rule.StartTime, rule.EndTime, rule.ID); err != nil {
			return err
		}

		delBrackets := fmt.Sprintf("DELETE FROM pricing_brackets WHERE rule_id = %s", database.Placeholder(r.cfg, 1))
		if _, err := tx.Exec(ctx, delBrackets, rule.ID); err != nil {
			return err
		}

		for i := range brackets {
			brackets[i].ID = uuid.NewString()
			brackets[i].RuleID = rule.ID
			if err := tx.Insert(ctx, pricingBracketsTable, &brackets[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PricingRepository) Delete(ctx context.Context, ruleID string) error {
	return r.db.Transaction(ctx, func(tx ksql.Provider) error {
		delBrackets := fmt.Sprintf("DELETE FROM pricing_brackets WHERE rule_id = %s", database.Placeholder(r.cfg, 1))
		if _, err := tx.Exec(ctx, delBrackets, ruleID); err != nil {
			return err
		}
		delRule := fmt.Sprintf("DELETE FROM pricing_rules WHERE id = %s", database.Placeholder(r.cfg, 1))
		_, err := tx.Exec(ctx, delRule, ruleID)
		return err
	})
}

// CloneToAllDays copies a rule (city/category/time window/brackets) into
// the other six days of the week, so setting up a full week only means
// configuring one day and cloning it — matching weekday configs stay
// independent afterwards (editing one clone doesn't touch the others).
func (r *PricingRepository) CloneToAllDays(ctx context.Context, ruleID string) error {
	original, err := r.FindByID(ctx, ruleID)
	if err != nil {
		return err
	}

	for day := 0; day <= 6; day++ {
		if day == original.DayOfWeek {
			continue
		}
		clone := &models.PricingRule{
			ID:         uuid.NewString(),
			CityID:     original.CityID,
			CategoryID: original.CategoryID,
			DayOfWeek:  day,
			StartTime:  original.StartTime,
			EndTime:    original.EndTime,
			CreatedAt:  time.Now().UTC(),
		}
		brackets := make([]models.PricingBracket, len(original.Brackets))
		for i, b := range original.Brackets {
			brackets[i] = models.PricingBracket{KmFrom: b.KmFrom, KmTo: b.KmTo, Price: b.Price}
		}
		if err := r.Create(ctx, clone, brackets); err != nil {
			return err
		}
	}
	return nil
}
