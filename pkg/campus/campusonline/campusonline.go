package campusonline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"golang.org/x/oauth2/clientcredentials"

	genaccount "github.com/TUM-Dev/gocast/pkg/campus/campusonline/gen/account"
	gencourse "github.com/TUM-Dev/gocast/pkg/campus/campusonline/gen/course"
	"github.com/TUM-Dev/gocast/pkg/campus/model"
)

const (
	defaultTokenURL      = "https://review.campus.tum.de/RSYSTEM/co/co-tm-core/swagger/token"
	defaultAPIURL        = "https://review.campus.tum.de/RSYSTEM/co/co-tm-core/course/api"
	defaultAccountAPIURL = "https://review.campus.tum.de/RSYSTEM/co/co-auth/account/api"

	// usernameBatchSize is the maximum number of person UIDs sent per account API request.
	usernameBatchSize = 100
)

// naiveTimestampRE matches ISO 8601 datetime strings that have no timezone suffix
// (no trailing Z, +HH:MM, or -HH:MM), so we can append Z to make them valid RFC3339.
var naiveTimestampRE = regexp.MustCompile(`"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?)"`)

// utcNormalizingTransport rewrites timezone-less timestamps in JSON responses
// to UTC (appends Z) before the generated decoder processes them.
type utcNormalizingTransport struct {
	base http.RoundTripper
}

func (t *utcNormalizingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
		return nil, closeErr
	}
	if err != nil {
		return nil, err
	}
	fixed := naiveTimestampRE.ReplaceAll(body, []byte(`"${1}Z"`))
	resp.Body = io.NopCloser(bytes.NewReader(fixed))
	resp.ContentLength = int64(len(fixed))
	return resp, nil
}

// Service is a client for the CAMPUSonline API. Use NewService to create an instance.
type Service struct {
	clientID      string
	clientSecret  string
	tokenURL      string
	apiURL        string
	accountAPIURL string
}

// NewService creates a new Service using OAuth2 client credentials.
// Defaults to the TUM review environment; use WithURLs / WithAccountURL to override.
func NewService(clientID string, clientSecret string) *Service {
	return &Service{
		clientID:      clientID,
		clientSecret:  clientSecret,
		tokenURL:      defaultTokenURL,
		apiURL:        defaultAPIURL,
		accountAPIURL: defaultAccountAPIURL,
	}
}

// WithURLs overrides the course API and token base URLs (e.g. for non-review environments).
// apiURL is the course API root; tokenURL is the OAuth2 token endpoint.
func (s *Service) WithURLs(apiURL, tokenURL string) *Service {
	s.apiURL = apiURL
	s.tokenURL = tokenURL
	return s
}

// WithAccountURL overrides the account API base URL.
func (s *Service) WithAccountURL(accountAPIURL string) *Service {
	s.accountAPIURL = accountAPIURL
	return s
}

func (s *Service) newAPIClient() *gencourse.APIClient {
	tokenCfg := clientcredentials.Config{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		TokenURL:     s.tokenURL,
		Scopes:       []string{"co-course.read"},
	}
	oauthClient := tokenCfg.Client(context.Background())
	oauthClient.Transport = &utcNormalizingTransport{base: oauthClient.Transport}

	apiCfg := gencourse.NewConfiguration()
	apiCfg.HTTPClient = oauthClient
	apiCfg.Servers = gencourse.ServerConfigurations{{URL: s.apiURL}}
	return gencourse.NewAPIClient(apiCfg)
}

func (s *Service) newAccountAPIClient() *genaccount.APIClient {
	tokenCfg := clientcredentials.Config{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		TokenURL:     s.tokenURL,
		Scopes:       []string{"co-account.read"},
	}
	oauthClient := tokenCfg.Client(context.Background())
	oauthClient.Transport = &utcNormalizingTransport{base: oauthClient.Transport}

	apiCfg := genaccount.NewConfiguration()
	apiCfg.HTTPClient = oauthClient
	apiCfg.Servers = genaccount.ServerConfigurations{{URL: s.accountAPIURL}}
	return genaccount.NewAPIClient(apiCfg)
}

// fetchUsernames looks up the username for each personUID via the account API in batches.
// It returns a map of personUID → username. Missing entries mean no account was found.
func (s *Service) fetchUsernames(ctx context.Context, personUIDs []string) (map[string]string, error) {
	client := s.newAccountAPIClient()
	result := make(map[string]string, len(personUIDs))

	for i := 0; i < len(personUIDs); i += usernameBatchSize {
		end := i + usernameBatchSize
		if end > len(personUIDs) {
			end = len(personUIDs)
		}
		batch := personUIDs[i:end]

		page, _, err := client.UsersAPI.
			UsersRestServiceGetUsers(ctx).
			PersonUid(batch).
			Execute()
		if err != nil {
			return nil, fmt.Errorf("fetch usernames batch %d: %w", i/usernameBatchSize, err)
		}

		for _, user := range page.Items {
			if user.PersonUid == nil {
				continue
			}
			// Use the first account that has a non-empty username.
			for _, acc := range user.Accounts {
				if acc.Username != nil && *acc.Username != "" {
					result[*user.PersonUid] = *acc.Username
					break
				}
			}
		}
	}
	return result, nil
}

// GetCourse fetches a single course by its identifier (e.g. CO course UID or numeric ID).
func (s *Service) GetCourse(identifier string) (*model.Course, error) {
	client := s.newAPIClient()

	resource, _, err := client.CourseServiceAPI.
		CourseRestServiceGetCourse(context.Background(), identifier).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("get course %q: %w", identifier, err)
	}

	return &model.Course{
		UID:             resource.Uid,
		Title:           resource.Title.Value,
		SemesterKey:     resource.SemesterKey,
		CourseTypeKey:   resource.CourseTypeKey,
		OrganisationUID: resource.OrganisationUid,
		SemesterHours:   resource.SemesterHours,
	}, nil
}

// GetCourseRegistrations fetches all registrations for the given course UID, including
// participant claims (name, email, matriculation number) and their CAMPUSonline username
// resolved via the account API.
func (s *Service) GetCourseRegistrations(courseUID string) ([]model.CourseRegistration, error) {
	client := s.newAPIClient()

	result, _, err := client.CourseRegistrationServiceAPI.
		CourseRegistrationRestServiceGetCourseRegistrations(context.Background()).
		CourseUid([]string{courseUID}).
		Claim([]gencourse.SelectableClaim{gencourse.SELECTABLECLAIM_CO_CLAIM_ALL}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("get course registrations for %q: %w", courseUID, err)
	}

	registrations := make([]model.CourseRegistration, 0, len(result.Items))
	personUIDs := make([]string, 0, len(result.Items))

	for _, item := range result.Items {
		reg := model.CourseRegistration{
			UID:            item.Uid,
			CourseUID:      item.CourseUid,
			CourseGroupUID: item.CourseGroupUid,
			PersonUID:      item.PersonUid,
			LastModifiedAt: item.LastModifiedAt,
		}
		if claims := item.PersonClaims; claims != nil {
			reg.GivenName = claims.GivenName
			reg.Surname = claims.Surname
			reg.MatriculationNr = claims.MatriculationNumber
			if claims.EmailStudent != nil {
				reg.Email = *claims.EmailStudent
			}
		}
		registrations = append(registrations, reg)
		personUIDs = append(personUIDs, item.PersonUid)
	}

	usernames, err := s.fetchUsernames(context.Background(), personUIDs)
	if err != nil {
		return nil, fmt.Errorf("get usernames for course %q registrations: %w", courseUID, err)
	}
	for i := range registrations {
		registrations[i].Username = usernames[registrations[i].PersonUID]
	}

	return registrations, nil
}
