package manager

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/acorn-io/baaah/pkg/randomtoken"
	"github.com/acorn-io/runtime/pkg/config"
	"github.com/pkg/browser"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/strings/slices"
)

func IsManager(cfg *config.CLIConfig, address string) (bool, error) {
	if slices.Contains(cfg.AcornServers, address) {
		return true, nil
	}

	req, err := http.NewRequest(http.MethodGet, toDiscoverURL(address), nil)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if !strings.Contains(string(data), "TokenRequest") {
		return false, nil
	}

	cfg.AcornServers = append(cfg.AcornServers, address)
	return true, cfg.Save()
}

func Projects(ctx context.Context, address, token string) ([]string, error) {
	memberships := &membershipList{}
	result := sets.NewString()
	err := httpGet(ctx, toProjectMembershipURL(address), token, memberships)
	if err != nil {
		return nil, err
	}
	for _, membership := range memberships.Items {
		if membership.AccountEndpointURL != "" {
			result.Insert(fmt.Sprintf("%s/%s/%s", address, membership.AccountName, membership.ProjectName))
		}
	}
	return result.List(), nil
}

func ProjectURL(ctx context.Context, serverAddress, accountName, token string) (url string, err error) {
	obj := &account{}
	if err := httpGet(ctx, toAccountURL(serverAddress, accountName), token, obj); err != nil {
		return "", err
	}
	if obj.Status.EndpointURL == "" {
		return "", fmt.Errorf("failed to find endpoint URL for account %s, account may still be provisioning", accountName)
	}
	return obj.Status.EndpointURL, nil
}

func Login(ctx context.Context, password, address string) (user string, pass string, err error) {
	if password == "" {
		password, err = randomtoken.Generate()
		if err != nil {
			return "", "", err
		}

		url := toLoginURL(address, password)
		_ = browser.OpenURL(url)
		fmt.Printf("\nNavigate your browser to %s and login\n", url)
	}

	tokenRequestURL := toTokenRequestURL(address, password)
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-timeout:
			return "", "", fmt.Errorf("timeout getting authentication token")
		default:
		}

		tokenRequest := &tokenRequest{}
		if err := httpGet(ctx, tokenRequestURL, "", tokenRequest); err == nil {
			if tokenRequest.Status.Expired {
				return "", "", fmt.Errorf("token request has expired, please try to login again")
			}
			if tokenRequest.Status.Token != "" {
				httpDelete(ctx, tokenRequestURL, tokenRequest.Status.Token)
				return tokenRequest.Spec.AccountName, tokenRequest.Status.Token, nil
			} else {
				logrus.Debugf("tokenRequest.Status.Token is empty")
			}
		} else {
			logrus.Debugf("error getting tokenrequest: %v", err)
		}

		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
	}
}

func DefaultProject(ctx context.Context, address, user, token string) (string, error) {
	projects, err := Projects(ctx, address, token)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", err
	}
	return projects[0], nil
}
