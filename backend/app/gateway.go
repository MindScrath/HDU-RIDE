package app

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func workspaceGateway(db *pgxpool.Pool, cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		workspaceID := c.Param("workspaceID")
		if workspaceID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
			return
		}

		var workspace Workspace
		err := db.QueryRow(c.Request.Context(), `
select id, user_id, class_id, assignment_id, pod_name, service_name, pvc_name, status, last_seen_at, created_at
from workspaces where id=$1 and status <> 'stopped'
`, workspaceID).Scan(&workspace.ID, &workspace.UserID, &workspace.ClassID, &workspace.AssignmentID, &workspace.PodName, &workspace.ServiceName, &workspace.PVCName, &workspace.Status, &workspace.LastSeenAt, &workspace.CreatedAt)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
			return
		}
		if workspace.UserID != user.ID && !isAdmin(user) {
			c.JSON(http.StatusForbidden, gin.H{"error": "workspace forbidden"})
			return
		}

		target := &url.URL{
			Scheme: "http",
			Host:   workspace.ServiceName + "." + cfg.K8sNamespace + ".svc.cluster.local:8787",
		}
		prefix := "/ide/s/" + workspace.ID
		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = target.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.Header.Set("X-RStudio-Root-Path", prefix)
			req.Header.Set("X-Forwarded-Prefix", prefix)
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("X-Frame-Options")
			setCookies := resp.Header.Values("Set-Cookie")
			if len(setCookies) > 0 {
				resp.Header.Del("Set-Cookie")
				for _, raw := range setCookies {
					parts := strings.Split(raw, ";")
					next := make([]string, 0, len(parts)+1)
					for _, part := range parts {
						if strings.HasPrefix(strings.ToLower(strings.TrimSpace(part)), "path=") {
							continue
						}
						next = append(next, part)
					}
					next = append(next, " Path="+prefix+"/")
					resp.Header.Add("Set-Cookie", strings.Join(next, ";"))
				}
			}
			if location := rewriteGatewayLocation(resp.Header.Get("Location"), prefix, target.Host); location != "" {
				resp.Header.Set("Location", location)
			}
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			log.Printf("workspace gateway error workspace=%s target=%s err=%v", workspace.ID, target.Host, err)
			http.Error(w, "workspace gateway error", http.StatusBadGateway)
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func rewriteGatewayLocation(location, prefix, upstreamHost string) string {
	if location == "" {
		return ""
	}
	if strings.HasPrefix(location, "/") {
		return prefixedGatewayPath(location, prefix)
	}
	u, err := url.Parse(location)
	if err != nil || u.Host != upstreamHost {
		return location
	}
	path := prefixedGatewayPath(u.EscapedPath(), prefix)
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path
}

func prefixedGatewayPath(path, prefix string) string {
	if path == "" {
		path = "/"
	}
	if path == prefix || strings.HasPrefix(path, prefix+"/") {
		return path
	}
	return prefix + path
}
