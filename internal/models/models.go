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
	ID            string    `ksql:"id" json:"id"`
	Name          string    `ksql:"name" json:"name"`
	Email         string    `ksql:"email" json:"email"`
	Phone         string    `ksql:"phone" json:"phone"`
	CPF           string    `ksql:"cpf" json:"cpf"`
	BirthDate     string    `ksql:"birth_date" json:"birthDate"`
	PhotoURL      string    `ksql:"photo_url" json:"photoUrl,omitempty"`
	PasswordHash  string    `ksql:"password_hash" json:"-"`
	CreditBalance float64   `ksql:"credit_balance" json:"creditBalance"`
	// PixKey is always the CANONICAL key Woovi returns from EnsureRecipient,
	// auto-provisioned (from CPF) on first login and changeable any time via
	// AuthHandler.SetPixKey — same convention as models.Driver.PixKey.
	PixKey    string    `ksql:"pix_key" json:"pixKey,omitempty"`
	GoogleSub *string   `ksql:"google_sub" json:"-"`
	CreatedAt time.Time `ksql:"created_at" json:"createdAt"`

	// GoogleLinked is set by the handler after loading the row — never a
	// column itself — so the client can show "linked" state without ever
	// seeing the actual Google subject id.
	GoogleLinked bool `json:"googleLinked"`
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
	// PixKey is always the CANONICAL key Woovi returns from EnsureRecipient,
	// never the raw value the driver typed — see internal/woovi. Empty means
	// she hasn't registered one yet, so her revenue wallet has no subaccount.
	PixKey    string    `ksql:"pix_key" json:"pixKey,omitempty"`
	GoogleSub *string   `ksql:"google_sub" json:"-"`
	CreatedAt time.Time `ksql:"created_at" json:"createdAt"`

	// Populated by the repository, never stored as a column directly.
	Categories []Category `json:"categories,omitempty"`
	// EtaMin is set only when this Driver is attached to a specific ride
	// (see RideHandler.attachDriver) — minutes from her current location to
	// the ride's pickup point. Nil outside that context.
	EtaMin *int `json:"etaMin,omitempty"`
	// GoogleLinked is set by the handler after loading the row — see
	// models.User.GoogleLinked doc.
	GoogleLinked bool `json:"googleLinked"`
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

type WalletTransactionType string

const (
	WalletTransactionRideEarning   WalletTransactionType = "ride_earning"
	WalletTransactionWithdrawal    WalletTransactionType = "withdrawal"
	WalletTransactionWithdrawalFee WalletTransactionType = "withdrawal_fee"
)

// WalletTransaction is an audit-only entry for a driver's revenue wallet —
// the spendable balance itself always comes live from Woovi, never summed
// from this table. A row is written even when crediting a ride's earning is
// skipped or fails, so that gap stays visible instead of silent.
type WalletTransaction struct {
	ID                    string    `ksql:"id" json:"id"`
	DriverID              string    `ksql:"driver_id" json:"driverId"`
	Type                  string    `ksql:"type" json:"type"`
	Amount                float64   `ksql:"amount" json:"amount"`
	RideID                *string   `ksql:"ride_id" json:"rideId,omitempty"`
	ProviderTransactionID *string   `ksql:"provider_transaction_id" json:"providerTransactionId,omitempty"`
	Note                  string    `ksql:"note" json:"note,omitempty"`
	CreatedAt             time.Time `ksql:"created_at" json:"createdAt"`
}

type CreditTopupStatus string

const (
	CreditTopupPending CreditTopupStatus = "pending"
	CreditTopupPaid    CreditTopupStatus = "paid"
	CreditTopupExpired CreditTopupStatus = "expired"
)

// CreditTopup tracks a driver's prepaid-credit top-up charge — id is
// Woovi's correlationID, so the webhook can look up "which driver, how
// much" from a bare correlationID alone.
type CreditTopup struct {
	ID          string     `ksql:"id" json:"id"`
	DriverID    string     `ksql:"driver_id" json:"driverId"`
	AmountCents int        `ksql:"amount_cents" json:"amountCents"`
	Status      string     `ksql:"status" json:"status"`
	CreatedAt   time.Time  `ksql:"created_at" json:"createdAt"`
	PaidAt      *time.Time `ksql:"paid_at" json:"paidAt,omitempty"`
}

// UserCreditTransaction mirrors CreditTransaction for a passenger's
// spend-only prepaid credit (pays for ride prices, never withdrawable).
type UserCreditTransaction struct {
	ID           string    `ksql:"id" json:"id"`
	UserID       string    `ksql:"user_id" json:"userId"`
	Type         string    `ksql:"type" json:"type"`
	Amount       float64   `ksql:"amount" json:"amount"`
	RideID       *string   `ksql:"ride_id" json:"rideId,omitempty"`
	BalanceAfter float64   `ksql:"balance_after" json:"balanceAfter"`
	Note         string    `ksql:"note" json:"note,omitempty"`
	CreatedAt    time.Time `ksql:"created_at" json:"createdAt"`
}

const (
	UserCreditTopupType      = "topup"
	UserCreditRidePayment    = "ride_payment"
	UserCreditAdjustmentType = "adjustment"
)

// UserCreditTopup mirrors CreditTopup for a passenger's credit top-up.
type UserCreditTopup struct {
	ID          string     `ksql:"id" json:"id"`
	UserID      string     `ksql:"user_id" json:"userId"`
	AmountCents int        `ksql:"amount_cents" json:"amountCents"`
	Status      string     `ksql:"status" json:"status"`
	CreatedAt   time.Time  `ksql:"created_at" json:"createdAt"`
	PaidAt      *time.Time `ksql:"paid_at" json:"paidAt,omitempty"`
}

// WooviSettings holds the Woovi (PIX) gateway credentials and the
// platform's own Pix key — a single configuration row edited from the
// admin panel's financial section, same shape as PaymentSettings.
type WooviSettings struct {
	ID                  int    `ksql:"id" json:"id"`
	Environment         string `ksql:"environment" json:"environment"`
	AppID               string `ksql:"app_id" json:"-"`
	WebhookSecret       string `ksql:"webhook_secret" json:"-"`
	WebhookPublicKeyB64 string `ksql:"webhook_public_key_b64" json:"-"`
	PlatformPixKey      string `ksql:"platform_pix_key" json:"platformPixKey"`
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
	// CommissionRate overrides PlatformSettings.CommissionRate for rides
	// whose origin matches this city; nil means "use the platform default".
	CommissionRate *float64 `ksql:"commission_rate" json:"commissionRate,omitempty"`
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
	PaymentMethod *string `ksql:"payment_method" json:"paymentMethod,omitempty"`
	Status        string  `ksql:"status" json:"status"`
	Rating        *int    `ksql:"rating" json:"rating,omitempty"`
	// Note is a short free-text message the passenger can leave for the
	// driver when requesting the ride (building access code, "toque a
	// campainha", etc.) — nil for the common case where she left it blank.
	Note      *string   `ksql:"note" json:"note,omitempty"`
	CreatedAt time.Time `ksql:"created_at" json:"createdAt"`

	// Populated by the handler after loading driver_id/user_id — never a
	// column directly (no ksql tag). Driver is attached for the passenger's
	// own view of a ride; User (the passenger) is attached for the driver's
	// view once she's accepted — never during the pre-accept offer, which
	// only ever needs pickup/destination to decide.
	Driver *Driver `json:"driver,omitempty"`
	User   *User   `json:"user,omitempty"`
}

const (
	RideMessageSenderUser   = "user"
	RideMessageSenderDriver = "driver"
)

// RideMessage is one line of the in-ride chat between passenger and driver —
// open while the ride has a driver and isn't completed/cancelled yet (the
// handler enforces that), kept as a read-only transcript afterward.
type RideMessage struct {
	ID         string    `ksql:"id" json:"id"`
	RideID     string    `ksql:"ride_id" json:"rideId"`
	SenderType string    `ksql:"sender_type" json:"senderType"`
	SenderID   string    `ksql:"sender_id" json:"senderId"`
	Body       string    `ksql:"body" json:"body"`
	CreatedAt  time.Time `ksql:"created_at" json:"createdAt"`
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

// DashboardStats is a simple summary of ride activity over a date range
// (or all-time when the range is unbounded) for the admin home page.
type DashboardStats struct {
	TotalRides          int     `ksql:"total_rides" json:"totalRides"`
	CompletedRides      int     `ksql:"completed_rides" json:"completedRides"`
	CancelledRides      int     `ksql:"cancelled_rides" json:"cancelledRides"`
	TotalDriverEarnings float64 `ksql:"total_driver_earnings" json:"totalDriverEarnings"`
	TotalPlatformFee    float64 `ksql:"total_platform_fee" json:"totalPlatformFee"`
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

// WebhookEventRecord dedups inbound webhook deliveries: (provider, event_key)
// is unique, so a redelivered event is silently dropped rather than
// double-processed (e.g. double-crediting a top-up).
type WebhookEventRecord struct {
	ID        string    `ksql:"id" json:"id"`
	Provider  string    `ksql:"provider" json:"provider"`
	EventKey  string    `ksql:"event_key" json:"eventKey"`
	Payload   string    `ksql:"payload" json:"-"`
	CreatedAt time.Time `ksql:"created_at" json:"createdAt"`
}

// AuditLogEntry records one admin action that touched money or a payout
// destination — who did it, what, when. AdminName is denormalized (kept
// even if the admin account is later removed) so the log stays readable on
// its own.
type AuditLogEntry struct {
	ID         string    `ksql:"id" json:"id"`
	AdminID    string    `ksql:"admin_id" json:"adminId"`
	AdminName  string    `ksql:"admin_name" json:"adminName"`
	Action     string    `ksql:"action" json:"action"`
	TargetType string    `ksql:"target_type" json:"targetType"`
	TargetID   string    `ksql:"target_id" json:"targetId"`
	Details    string    `ksql:"details" json:"details,omitempty"`
	CreatedAt  time.Time `ksql:"created_at" json:"createdAt"`
}

type PixKeyChangeStatus string

const (
	PixKeyChangePending  PixKeyChangeStatus = "pending"
	PixKeyChangeApproved PixKeyChangeStatus = "approved"
	PixKeyChangeRejected PixKeyChangeStatus = "rejected"
)

const (
	PixKeyOwnerDriver = "driver"
	PixKeyOwnerUser   = "user"
)

// PixKeyChangeRequest is a driver's or passenger's request to change her
// Pix key — never applied automatically; an admin must approve it first
// (see PixKeyChangeStatus doc / migration 000025 for why).
type PixKeyChangeRequest struct {
	ID          string     `ksql:"id" json:"id"`
	OwnerType   string     `ksql:"owner_type" json:"ownerType"`
	OwnerID     string     `ksql:"owner_id" json:"ownerId"`
	OldPixKey   string     `ksql:"old_pix_key" json:"oldPixKey,omitempty"`
	NewPixKey   string     `ksql:"new_pix_key" json:"newPixKey"`
	Status      string     `ksql:"status" json:"status"`
	RequestedAt time.Time  `ksql:"requested_at" json:"requestedAt"`
	ReviewedAt  *time.Time `ksql:"reviewed_at" json:"reviewedAt,omitempty"`
	ReviewedBy  *string    `ksql:"reviewed_by" json:"reviewedBy,omitempty"`
	Note        string     `ksql:"note" json:"note,omitempty"`

	// Populated by the admin handler, never stored as a column directly.
	OwnerName string `json:"ownerName,omitempty"`
}

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
