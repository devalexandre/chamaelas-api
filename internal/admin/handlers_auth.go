package admin

import (
	"net/http"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

func (m *Module) LoginPage(c echo.Context) error {
	return render(c, "", "login.html", map[string]any{})
}

func (m *Module) Login(c echo.Context) error {
	email := c.FormValue("email")
	password := c.FormValue("password")

	admin, err := m.admins.FindByEmail(c.Request().Context(), email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return render(c, "", "login.html", map[string]any{
			"Error": "E-mail ou senha incorretos.",
			"Email": email,
		})
	}

	sess, _ := session.Get(sessionName, c)
	sess.Values["adminID"] = admin.ID
	sess.Options = &sessionsOptions
	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/admin")
}

func (m *Module) Logout(c echo.Context) error {
	sess, _ := session.Get(sessionName, c)
	sess.Values["adminID"] = nil
	sess.Options.MaxAge = -1
	_ = sess.Save(c.Request(), c.Response())
	return c.Redirect(http.StatusSeeOther, "/admin/login")
}
