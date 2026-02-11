package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/storage"
)

type Config struct {
	// Optional. If empty, the policyserv API will be disabled.
	ApiKey        string
	JoinViaServer string
}

type Api struct {
	storage          storage.PersistentStorage
	hs               *homeserver.Homeserver
	communityManager *community.Manager
	apiKey           string
	joinViaServer    string
}

func NewApi(config *Config, storage storage.PersistentStorage, hs *homeserver.Homeserver, communityManager *community.Manager) (*Api, error) {
	return &Api{
		storage:          storage,
		hs:               hs,
		communityManager: communityManager,
		apiKey:           config.ApiKey,
		joinViaServer:    config.JoinViaServer,
	}, nil
}

func (a *Api) httpRequestHandler(upstream func(api *Api, w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream(a, w, r)
	})
}

func (a *Api) httpAuthenticatedRequestHandler(upstream func(api *Api, w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+a.apiKey {
			defer metrics.RecordHttpResponse(r.Method, "httpAuthenticatedRequestHandler", http.StatusUnauthorized)
			homeserver.MatrixHttpError(w, http.StatusUnauthorized, "M_UNAUTHORIZED", "Not allowed")
			return
		}

		upstream(a, w, r)
	})
}

func (a *Api) httpCommunityAuthenticatedRequestHandler(upstream func(api *Api, community *storage.StoredCommunity, w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) <= len("Bearer ") {
			defer metrics.RecordHttpResponse(r.Method, "httpCommunityAuthenticatedRequestHandler", http.StatusUnauthorized)
			homeserver.MatrixHttpError(w, http.StatusUnauthorized, "M_UNAUTHORIZED", "Not allowed")
			return
		}

		// Set a quick timeout that only affects the community lookup/authentication
		fastContext, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		accessToken := authHeader[len("Bearer "):]
		community, err := a.storage.GetCommunityByAccessToken(fastContext, accessToken)
		if err != nil {
			log.Println(err)
			defer metrics.RecordHttpResponse(r.Method, "httpCommunityAuthenticatedRequestHandler", http.StatusInternalServerError)
			homeserver.MatrixHttpError(w, http.StatusInternalServerError, "M_UNKNOWN", "Server error")
			return
		}
		if community == nil {
			defer metrics.RecordHttpResponse(r.Method, "httpCommunityAuthenticatedRequestHandler", http.StatusUnauthorized)
			homeserver.MatrixHttpError(w, http.StatusUnauthorized, "M_UNAUTHORIZED", "Not allowed")
			return
		}

		upstream(a, community, w, r)
	})
}

func (a *Api) BindTo(mux *http.ServeMux) error {
	mux.Handle("/", a.httpRequestHandler(httpCatchAll))
	mux.Handle("/health", a.httpRequestHandler(httpHealth))
	mux.Handle("/ready", a.httpRequestHandler(httpReady))

	if a.apiKey != "" {
		log.Println("Enabling policyserv API")
		mux.Handle("/api/v1/rooms", a.httpAuthenticatedRequestHandler(httpGetRoomsApi))
		mux.Handle("/api/v1/set_room_moderator", a.httpAuthenticatedRequestHandler(httpSetModeratorApi))
		mux.Handle("/api/v1/rooms/{id}", a.httpAuthenticatedRequestHandler(httpGetRoomApi))
		mux.Handle("/api/v1/rooms/{roomId}/join", a.httpAuthenticatedRequestHandler(httpAddRoomApi))
		mux.Handle("/api/v1/communities/new", a.httpAuthenticatedRequestHandler(httpCreateCommunityApi))
		mux.Handle("/api/v1/communities/{id}", a.httpAuthenticatedRequestHandler(httpGetCommunityApi))
		mux.Handle("/api/v1/communities/{id}/config", a.httpAuthenticatedRequestHandler(httpSetCommunityConfigApi))
		mux.Handle("/api/v1/communities/{id}/rotate_access_token", a.httpAuthenticatedRequestHandler(httpRotateCommunityAccessTokenApi))
		mux.Handle("/api/v1/instance/community_config", a.httpAuthenticatedRequestHandler(httpGetInstanceConfigApi))
		mux.Handle("/api/v1/sources/muninn/set_member_directory_event", a.httpAuthenticatedRequestHandler(httpSetMuninnSourceData))
		mux.Handle("/api/v1/keyword_templates/{name}", a.httpAuthenticatedRequestHandler(httpKeywordTemplates))
	}

	return nil
}
