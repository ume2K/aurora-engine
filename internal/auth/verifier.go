package auth

import "gocore/pkg/framework"

func FrameworkTokenVerifier(jwtManager *JWTManager) framework.TokenVerifier {
	return func(rawToken string) (framework.AuthInfo, error) {
		claims, err := jwtManager.VerifyToken(rawToken)
		if err != nil {
			return framework.AuthInfo{}, err
		}
		return framework.AuthInfo{
			UserID: claims.UserID,
			Role:   claims.Role,
		}, nil
	}
}
