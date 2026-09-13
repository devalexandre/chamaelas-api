package models

import "time"

// Address mirrors the frontend's Address type (frontend/src/types.ts) field
// for field, so rides can be sent to the app without any reshaping.
type Address struct {
	UF           string  `json:"uf"`
	City         string  `json:"city"`
	Street       string  `json:"street"`
	Number       string  `json:"number,omitempty"`
	Neighborhood string  `json:"neighborhood,omitempty"`
	CEP          string  `json:"cep,omitempty"`
	Lat          float64 `json:"lat,omitempty"`
	Lng          float64 `json:"lng,omitempty"`
	Label        string  `json:"label"`
}

type User struct {
	ID           string    `ksql:"id" json:"id"`
	Name         string    `ksql:"name" json:"name"`
	Email        string    `ksql:"email" json:"email"`
	Phone        string    `ksql:"phone" json:"phone"`
	CPF          string    `ksql:"cpf" json:"cpf"`
	BirthDate    string    `ksql:"birth_date" json:"birthDate"`
	PhotoURL     string    `ksql:"photo_url" json:"photoUrl,omitempty"`
	PasswordHash string    `ksql:"password_hash" json:"-"`
	CreatedAt    time.Time `ksql:"created_at" json:"createdAt"`
}

type DriverStatus string

const (
	DriverStatusPending  DriverStatus = "pending"
	DriverStatusApproved DriverStatus = "approved"
	DriverStatusBlocked  DriverStatus = "blocked"
)

type Driver struct {
	ID                 string     `ksql:"id" json:"id"`
	Name               string     `ksql:"name" json:"name"`
	Email              string     `ksql:"email" json:"email"`
	Phone              string     `ksql:"phone" json:"phone"`
	CPF                string     `ksql:"cpf" json:"cpf"`
	CNH                string     `ksql:"cnh" json:"cnh"`
	BirthDate          string     `ksql:"birth_date" json:"birthDate"`
	PhotoURL           string     `ksql:"photo_url" json:"photoUrl,omitempty"`
	PasswordHash       string     `ksql:"password_hash" json:"-"`
	VehiclePlate       string     `ksql:"vehicle_plate" json:"vehiclePlate"`
	VehicleModel       string     `ksql:"vehicle_model" json:"vehicleModel"`
	VehicleColor       string     `ksql:"vehicle_color" json:"vehicleColor"`
	VehicleYear        string     `ksql:"vehicle_year" json:"vehicleYear"`
	Rating             float64    `ksql:"rating" json:"rating"`
	Status             string     `ksql:"status" json:"status"`
	IsOnline           bool       `ksql:"is_online" json:"isOnline"`
	Lat                *float64   `ksql:"lat" json:"lat,omitempty"`
	Lng                *float64   `ksql:"lng" json:"lng,omitempty"`
	LocationUpdatedAt  *time.Time `ksql:"location_updated_at" json:"locationUpdatedAt,omitempty"`
	BillingMode        string     `ksql:"billing_mode" json:"billingMode"`
	CreditBalance      float64    `ksql:"credit_balance" json:"creditBalance"`
	BankCode           string     `ksql:"bank_code" json:"bankCode,omitempty"`
	BankBranch         string     `ksql:"bank_branch" json:"bankBranch,omitempty"`
	BankAccountNumber  string     `ksql:"bank_account_number" json:"bankAccountNumber,omitempty"`
	BankAccountType    string     `ksql:"bank_account_type" json:"bankAccountType,omitempty"`
	PagarmeRecipientID string     `ksql:"pagarme_recipient_id" json:"pagarmeRecipientId,omitempty"`
	CreatedAt          time.Time  `ksql:"created_at" json:"createdAt"`

	// Populated by the repository, never stored as a column directly.
	Categories []Category `json:"categories,omitempty"`
	// EtaMin is set only when this Driver is attached to a specific ride
	// (see RideHandler.attachDriver) — minutes from her current location to
	// the ride's pickup point. Nil outside that context.
	EtaMin *int `json:"etaMin,omitempty"`
}

// OnlineStaleness is how long IsOnline=true is trusted without a fresh
// location ping — the app pings every 20s while online, so a driver who
// crashed, lost connectivity, or force-quit instead of toggling offline
// otherwise leaves the flag stuck at true forever with no other signal.
const OnlineStaleness = 90 * time.Second

// IsReallyOnline is IsOnline narrowed by recency: the single definition of
// "online" every caller (admin dashboard, driver list, nearby-driver maps)
// should use, so a stale flag doesn't show a ghost driver as available.
func (d Driver) IsReallyOnline() bool {
	return d.IsOnline && d.LocationUpdatedAt != nil && time.Since(*d.LocationUpdatedAt) < OnlineStaleness
}

type BillingMode string

const (
	BillingModePerRide BillingMode = "per_ride"
	BillingModePrepaid BillingMode = "prepaid"
)

type CreditTransactionType string

const (
	CreditTransactionTopup      CreditTransactionType = "topup"
	CreditTransactionRideFee    CreditTransactionType = "ride_fee"
	CreditTransactionAdjustment CreditTransactionType = "adjustment"
)

type CreditTransaction struct {
	ID           string    `ksql:"id" json:"id"`
	DriverID     string    `ksql:"driver_id" json:"driverId"`
	Type         string    `ksql:"type" json:"type"`
	Amount       float64   `ksql:"amount" json:"amount"`
	RideID       *string   `ksql:"ride_id" json:"rideId,omitempty"`
	BalanceAfter float64   `ksql:"balance_after" json:"balanceAfter"`
	Note         string    `ksql:"note" json:"note,omitempty"`
	CreatedAt    time.Time `ksql:"created_at" json:"createdAt"`
}

type PlatformSettings struct {
	ID             int     `ksql:"id" json:"id"`
	CommissionRate float64 `ksql:"commission_rate" json:"commissionRate"`
	// MapPollSeconds is how often the apps' live maps (own position, nearby
	// drivers) refresh — configurable here instead of hardcoded per app.
	MapPollSeconds int `ksql:"map_poll_seconds" json:"mapPollSeconds"`
}

type Category struct {
	ID          string `ksql:"id" json:"id"`
	Name        string `ksql:"name" json:"name"`
	Description string `ksql:"description" json:"description"`
	Active      bool   `ksql:"active" json:"active"`
}

type City struct {
	ID     string `ksql:"id" json:"id"`
	Name   string `ksql:"name" json:"name"`
	UF     string `ksql:"uf" json:"uf"`
	Active bool   `ksql:"active" json:"active"`
}

type Admin struct {
	ID           string    `ksql:"id" json:"id"`
	Name         string    `ksql:"name" json:"name"`
	Email        string    `ksql:"email" json:"email"`
	PasswordHash string    `ksql:"password_hash" json:"-"`
	CreatedAt    time.Time `ksql:"created_at" json:"createdAt"`
}

type RideStatus string

const (
	RideStatusSearching  RideStatus = "searching"
	RideStatusAccepted   RideStatus = "accepted"
	RideStatusArrived    RideStatus = "arrived"
	RideStatusInProgress RideStatus = "in_progress"
	RideStatusCompleted  RideStatus = "completed"
	RideStatusCancelled  RideStatus = "cancelled"
)

type Ride struct {
	ID          string        `ksql:"id" json:"id"`
	UserID      string        `ksql:"user_id" json:"userId"`
	DriverID    *string       `ksql:"driver_id" json:"driverId,omitempty"`
	CategoryID  string        `ksql:"category_id" json:"categoryId"`
	Origin      JSON[Address] `ksql:"origin" json:"origin"`
	Destination JSON[Address] `ksql:"destination" json:"destination"`
	DistanceKm  float64       `ksql:"distance_km" json:"distanceKm"`
	DurationMin int           `ksql:"duration_min" json:"durationMin"`
	// Price is the total charged to the passenger: DriverEarning + PlatformFee.
	// The platform's commission is always added on top — a driver never has
	// it deducted from what she earns on the ride itself.
	Price         float64 `ksql:"price" json:"price"`
	DriverEarning float64 `ksql:"driver_earning" json:"driverEarning"`
	PlatformFee   float64 `ksql:"platform_fee" json:"platformFee"`
	// FinalPrice is set when the ride actually completes — nil for a ride
	// that's still in progress or was cancelled, since it never reached a
	// final value. Today it always matches Price (nothing adjusts the fare
	// yet), but it's the field to update if/when fare adjustments exist.
	FinalPrice *float64 `ksql:"final_price" json:"finalPrice,omitempty"`
	// AmountPaid is set once real payment capture exists; nil until then.
	AmountPaid *float64 `ksql:"amount_paid" json:"amountPaid,omitempty"`
	// PaymentMethod is set once a real charge goes through Pagar.me
	// ("pix", "credit_card", ...); stays nil until that integration exists.
	PaymentMethod *string   `ksql:"payment_method" json:"paymentMethod,omitempty"`
	Status        string    `ksql:"status" json:"status"`
	Rating        *int      `ksql:"rating" json:"rating,omitempty"`
	CreatedAt     time.Time `ksql:"created_at" json:"createdAt"`

	// Populated by the repository after loading driver_id. Has no ksql tag on
	// purpose: fields without one are ignored by ksql's struct scanning, so
	// this never gets treated as a column.
	Driver *Driver `json:"driver,omitempty"`
}

// PricingRule is a day-of-week + time-window scope (optionally narrowed to
// one city and/or category) that a set of PricingBrackets attach prices to.
type PricingRule struct {
	ID         string    `ksql:"id" json:"id"`
	CityID     *string   `ksql:"city_id" json:"cityId,omitempty"`
	CategoryID *string   `ksql:"category_id" json:"categoryId,omitempty"`
	DayOfWeek  int       `ksql:"day_of_week" json:"dayOfWeek"`
	StartTime  string    `ksql:"start_time" json:"startTime"`
	EndTime    string    `ksql:"end_time" json:"endTime"`
	CreatedAt  time.Time `ksql:"created_at" json:"createdAt"`

	// Populated by the repository, never stored as columns directly.
	CityName     string           `json:"cityName,omitempty"`
	CategoryName string           `json:"categoryName,omitempty"`
	Brackets     []PricingBracket `json:"brackets,omitempty"`
}

// PricingBracket's final price for a ride within [KmFrom, KmTo) is
// Price + PricePerKm*distanceKm — Price alone gives a flat bracket price
// (PricePerKm defaults to 0), PricePerKm alone gives a pure per-km rate
// (Price defaults to 0), and both together give "fixo + por km".
type PricingBracket struct {
	ID         string   `ksql:"id" json:"id"`
	RuleID     string   `ksql:"rule_id" json:"ruleId"`
	KmFrom     float64  `ksql:"km_from" json:"kmFrom"`
	KmTo       *float64 `ksql:"km_to" json:"kmTo,omitempty"`
	Price      float64  `ksql:"price" json:"price"`
	PricePerKm float64  `ksql:"price_per_km" json:"pricePerKm"`
}

// FinalPrice returns this bracket's price for a ride of the given distance.
func (b PricingBracket) FinalPrice(distanceKm float64) float64 {
	return b.Price + b.PricePerKm*distanceKm
}

var Weekdays = []string{"Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"}

// PaymentSettings holds the Pagar.me gateway credentials and the
// platform's own recipient ("nossa carteira") — a single configuration
// row edited from the admin panel's financial section.
type PaymentSettings struct {
	ID                  int    `ksql:"id" json:"id"`
	Environment         string `ksql:"environment" json:"environment"`
	PublicKey           string `ksql:"public_key" json:"publicKey"`
	SecretKey           string `ksql:"secret_key" json:"-"`
	PlatformRecipientID string `ksql:"platform_recipient_id" json:"platformRecipientId"`
}

// RideReportRow is one grouped row of a revenue report (by date, category,
// or payment method) — GroupKey holds whichever dimension the report is
// sliced by.
type RideReportRow struct {
	GroupKey           string  `ksql:"group_key" json:"groupKey"`
	RideCount          int     `ksql:"ride_count" json:"rideCount"`
	TotalPrice         float64 `ksql:"total_price" json:"totalPrice"`
	TotalDriverEarning float64 `ksql:"total_driver_earning" json:"totalDriverEarning"`
	TotalPlatformFee   float64 `ksql:"total_platform_fee" json:"totalPlatformFee"`
}

// RideDetailRow is one row of the "por data" report — every ride in a date
// range with who was involved and how much moved, not grouped/summed.
type RideDetailRow struct {
	ID            string    `ksql:"id" json:"id"`
	CreatedAt     time.Time `ksql:"created_at" json:"createdAt"`
	CustomerName  string    `ksql:"customer_name" json:"customerName"`
	DriverName    *string   `ksql:"driver_name" json:"driverName,omitempty"`
	PaymentMethod *string   `ksql:"payment_method" json:"paymentMethod,omitempty"`
	Price         float64   `ksql:"price" json:"price"`
	FinalPrice    *float64  `ksql:"final_price" json:"finalPrice,omitempty"`
	AmountPaid    *float64  `ksql:"amount_paid" json:"amountPaid,omitempty"`
	Status        string    `ksql:"status" json:"status"`
}

// RideLogRow is one row of the admin panel's live "Corridas" log — every
// ride regardless of status, with enough matching state (who it's currently
// offered to, when that offer expires) to see whether the matcher is
// actually working, not just whether a ride eventually completed.
type RideLogRow struct {
	ID           string    `ksql:"id" json:"id"`
	CreatedAt    time.Time `ksql:"created_at" json:"createdAt"`
	Status       string    `ksql:"status" json:"status"`
	CategoryID   string    `ksql:"category_id" json:"categoryId"`
	CustomerName string    `ksql:"customer_name" json:"customerName"`
	DriverName   *string   `ksql:"driver_name" json:"driverName,omitempty"`
	DeclineCount int       `ksql:"decline_count" json:"declineCount"`
	DistanceKm   float64   `ksql:"distance_km" json:"distanceKm"`
	Price        float64   `ksql:"price" json:"price"`
}

// GatewayFeeRate is what Pagar.me itself charges the platform for a given
// payment method — distinct from PlatformSettings.CommissionRate, which is
// what the platform charges drivers/passengers. Kept so real margins can be
// calculated once payments are wired up.
type GatewayFeeRate struct {
	ID            string  `ksql:"id" json:"id"`
	PaymentMethod string  `ksql:"payment_method" json:"paymentMethod"`
	Label         string  `ksql:"label" json:"label"`
	FeePercent    float64 `ksql:"fee_percent" json:"feePercent"`
	SortOrder     int     `ksql:"sort_order" json:"sortOrder"`
}

const (
	NotificationRecipientDriver = "driver"
	NotificationRecipientUser   = "user"
)

// Notification is a message the admin panel sent to one driver or one
// passenger — a broadcast to several recipients becomes one row per
// recipient. There's no push delivery yet: each app polls for its own
// unread rows.
type Notification struct {
	ID            string     `ksql:"id" json:"id"`
	RecipientType string     `ksql:"recipient_type" json:"recipientType"`
	RecipientID   string     `ksql:"recipient_id" json:"recipientId"`
	Title         string     `ksql:"title" json:"title"`
	Message       string     `ksql:"message" json:"message"`
	CreatedAt     time.Time  `ksql:"created_at" json:"createdAt"`
	ReadAt        *time.Time `ksql:"read_at" json:"readAt,omitempty"`
}
