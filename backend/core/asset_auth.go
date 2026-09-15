package core

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"noyo/core/store"
	"github.com/gogf/gf/v2/net/ghttp"
)

// The cookie is usable only for same-origin, read-only media routes. API writes
// continue to require explicit Authorization; no bearer is placed in a URL.
func setAssetCookie(r *ghttp.Request, token string, maxAge int) {
	http.SetCookie(r.Response.Writer, &http.Cookie{Name:"noyo_media", Value:token, Path:"/data/", HttpOnly:true, Secure:r.TLS!=nil, SameSite:http.SameSiteStrictMode, MaxAge:maxAge})
}

func recordUploadedAsset(r *ghttp.Request, path string) error {
	auth := requestAuthContext(r)
	if auth == nil { return errors.New("missing asset owner") }
	return store.DB.Create(&store.StoredAsset{Path:path, TenantID:auth.TenantID, ProjectID:auth.ProjectID, OwnerID:auth.UserID}).Error
}

func (s *Server) serveAuthorizedAsset(r *ghttp.Request, directory, prefix string, attachment bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead { r.Response.WriteStatus(http.StatusMethodNotAllowed); return }
	// Tenant logos are intentionally returned by the unauthenticated tenant
	// discovery endpoint. Only an exact current tenant-logo value is public;
	// every other image and every attachment remains authorization-gated.
	if !attachment && isPublicTenantLogo(r.URL.Path) {
		serveStoredFile(r,directory,prefix,false)
		return
	}
	if r.Header.Get("Authorization") == "" {
		if c,err:=r.Request.Cookie("noyo_media"); err==nil { r.Header.Set("Authorization","Bearer "+c.Value) }
	}
	// URL query bearer tokens are intentionally not accepted on these routes.
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Authorization")),"bearer ") { r.Response.WriteStatus(http.StatusUnauthorized); return }
	var asset store.StoredAsset
	found := store.DB.Where("path = ?",r.URL.Path).First(&asset).Error == nil
	if found {
		r.Header.Set("X-Current-Tenant-ID",strconv.FormatUint(uint64(asset.TenantID),10))
		r.Header.Set("X-Current-Project-ID",strconv.FormatUint(uint64(asset.ProjectID),10))
	}
	auth := authenticateRequest(r,s.Config.Auth.JWTSecret)
	if auth == nil { return }
	allowed := auth.IsSystemAdmin
	if found && !allowed && auth.TenantID == asset.TenantID {
		if asset.OwnerID != 0 && auth.UserID == asset.OwnerID { allowed=true }
		if asset.DeviceCode != "" && auth.HasPermission("device:list") {
			ok,err:=canReadDeviceByTagPermission(auth,currentDeviceTagScope(r),asset.DeviceCode)
			allowed=err==nil && ok
		}
	}
	// Legacy and shared work-order files must be proven through a readable
	// business reference; absence of provenance never means public access.
	if !allowed { allowed = canReadWorkOrderAsset(auth,r.URL.Path) }
	if !allowed { r.Response.WriteStatus(http.StatusNotFound); return }
	serveStoredFile(r,directory,prefix,attachment)
}

func isPublicTenantLogo(path string) bool {
	if store.DB == nil || !strings.HasPrefix(path, "/data/images/") { return false }
	var count int64
	return store.DB.Model(&store.Tenant{}).Where("logo = ?", path).Count(&count).Error == nil && count > 0
}
