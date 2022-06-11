package resolvers

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt"
	log "github.com/sirupsen/logrus"

	"github.com/authorizerdev/authorizer/server/graph/model"
	"github.com/authorizerdev/authorizer/server/memorystore"
	"github.com/authorizerdev/authorizer/server/parsers"
	"github.com/authorizerdev/authorizer/server/token"
	"github.com/authorizerdev/authorizer/server/utils"
)

// ValidateJwtTokenResolver is used to validate a jwt token without its rotation
// this can be used at API level (backend)
// it can validate:
// access_token
// id_token
// refresh_token
func ValidateJwtTokenResolver(ctx context.Context, params model.ValidateJWTTokenInput) (*model.ValidateJWTTokenResponse, error) {
	gc, err := utils.GinContextFromContext(ctx)
	if err != nil {
		log.Debug("Failed to get GinContext: ", err)
		return nil, err
	}

	tokenType := params.TokenType
	if tokenType != "access_token" && tokenType != "refresh_token" && tokenType != "id_token" {
		log.Debug("Invalid token type: ", tokenType)
		return nil, errors.New("invalid token type")
	}

	var claimRoles []string
	var claims jwt.MapClaims
	userID := ""
	nonce := ""
	// access_token and refresh_token should be validated from session store as well
	if tokenType == "access_token" || tokenType == "refresh_token" {
		claims, err = token.ParseJWTToken(params.Token)
		if err != nil {
			log.Debug("Failed to parse JWT token: ", err)
			return nil, err
		}
		userID = claims["sub"].(string)
		nonce, err = memorystore.Provider.GetUserSession(userID, params.Token)
		if err != nil || nonce == "" {
			log.Debug("Failed to get user session: ", err)
			return nil, errors.New("invalid token")
		}
	} else {
		// for ID token just parse jwt
		claims, err = token.ParseJWTToken(params.Token)
		if err != nil {
			log.Debug("Failed to parse JWT token: ", err)
			return nil, err
		}
		userID = claims["sub"].(string)
	}

	hostname := parsers.GetHost(gc)

	// we cannot validate sub and nonce in case of id_token as that token is not persisted in session store
	if userID != "" && nonce != "" {
		if ok, err := token.ValidateJWTClaims(claims, hostname, nonce, userID); !ok || err != nil {
			log.Debug("Failed to parse jwt token: ", err)
			return nil, errors.New("invalid claims")
		}
	} else {
		if ok, err := token.ValidateJWTTokenWithoutNonce(claims, hostname); !ok || err != nil {
			log.Debug("Failed to parse jwt token without nonce: ", err)
			return nil, errors.New("invalid claims")
		}
	}

	claimRolesInterface := claims["roles"]
	roleSlice := utils.ConvertInterfaceToSlice(claimRolesInterface)
	for _, v := range roleSlice {
		claimRoles = append(claimRoles, v.(string))
	}

	if params.Roles != nil && len(params.Roles) > 0 {
		for _, v := range params.Roles {
			if !utils.StringSliceContains(claimRoles, v) {
				log.Debug("Token does not have required role: ", v)
				return nil, fmt.Errorf(`unauthorized`)
			}
		}
	}
	return &model.ValidateJWTTokenResponse{
		IsValid: true,
	}, nil
}
