package apiv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func integrationBearer(raw []byte) context.Context {
	key := base64.RawURLEncoding.EncodeToString(raw)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+key))
}

func TestIntegrationAuthentication(t *testing.T) {
	raw := bytes.Repeat([]byte{7}, 32)
	hash := sha256.Sum256(raw)
	next := bytes.Repeat([]byte{8}, 32)
	nextHash := sha256.Sum256(next)
	integration := model.Integration{ID: 4, Name: "Course portal", ReturnURL: "https://example/callback"}
	ctrl := gomock.NewController(t)
	integrationDao := mock_dao.NewMockIntegrationDao(ctrl)
	gomock.InOrder(
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), hash[:]).Return(integration, nil),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), hash[:]).Return(model.Integration{}, gorm.ErrRecordNotFound),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), nextHash[:]).Return(integration, nil),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), nextHash[:]).Return(model.Integration{}, nil),
		integrationDao.EXPECT().GetIntegrationByAPIKeyHash(gomock.Any(), nextHash[:]).Return(model.Integration{}, errors.New("database unavailable")),
	)
	api := &API{dao: dao.DaoWrapper{IntegrationDao: integrationDao}, log: slog.Default()}
	info := &grpc.UnaryServerInfo{FullMethod: method(&protobuf.MetaService_ServiceDesc, "getIntegration")}
	_, err := api.resolveCaller(integrationBearer(raw), nil, info, func(ctx context.Context, _ any) (any, error) {
		return api.authorize(ctx, nil, info, func(ctx context.Context, _ any) (any, error) {
			resolved := ctx.Value(callerKey{}).(*caller)
			require.NotNil(t, resolved.integration)
			assert.Nil(t, resolved.user)
			assert.NoError(t, resolved.err)
			response, callErr := api.GetIntegration(ctx, &emptypb.Empty{})
			require.NoError(t, callErr)
			assert.Equal(t, uint32(4), response.Id)
			assert.Equal(t, "https://example/callback", response.ReturnUrl)
			return nil, nil
		})
	})
	require.NoError(t, err)

	_, err = api.resolveIntegration(integrationBearer(raw))
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "old key remained valid after rotation")
	_, err = api.resolveIntegration(integrationBearer(next))
	require.NoError(t, err)
	_, err = api.resolveIntegration(integrationBearer(next))
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "revoked key remained valid")

	_, err = api.resolveIntegration(integrationBearer([]byte("short")))
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = api.resolveIntegration(integrationBearer(next))
	assert.Equal(t, codes.Unknown, status.Code(err))
	assert.Equal(t, "could not authenticate integration", status.Convert(err).Message())
}

func TestIntegrationCredentialsStaySeparateFromUserCredentials(t *testing.T) {
	api := &API{log: slog.Default()}
	integrationMethod := &grpc.UnaryServerInfo{FullMethod: method(&protobuf.MetaService_ServiceDesc, "getIntegration")}
	userMethod := &grpc.UnaryServerInfo{FullMethod: method(&protobuf.UserService_ServiceDesc, "getUser")}
	for _, test := range []struct {
		name string
		ctx  context.Context
		info *grpc.UnaryServerInfo
	}{
		{"session cookie on integration method", metadata.NewIncomingContext(context.Background(), metadata.Pairs("grpcgateway-cookie", "jwt=session")), integrationMethod},
		{"human JWT on integration method", metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature")), integrationMethod},
		{"integration key on user method", integrationBearer(bytes.Repeat([]byte{7}, 32)), userMethod},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := api.resolveCaller(test.ctx, nil, test.info, func(ctx context.Context, req any) (any, error) {
				return api.authorize(ctx, req, test.info, func(context.Context, any) (any, error) {
					t.Fatal("unexpected access to protected method")
					return nil, nil
				})
			})
			assert.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func TestRedeemIntegrationAuthorizationHashesCanonicalValues(t *testing.T) {
	code, err := uuid.Parse("685aebd3-45ba-4d4c-91a5-b181932f2ff5")
	require.NoError(t, err)
	state := bytes.Repeat([]byte{4}, 32)
	codeHash, stateHash := sha256.Sum256(code[:]), sha256.Sum256(state)
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	integrationGrantDao.EXPECT().RedeemIntegrationAuthorizationCode(gomock.Any(), uint(7), codeHash[:], stateHash[:], gomock.Any()).Return(uint(123), uint(456), nil)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

	response, err := api.RedeemIntegrationAuthorization(ctx, &protobuf.RedeemIntegrationAuthorizationRequest{
		Code: code.String(), State: base64.RawURLEncoding.EncodeToString(state),
	})

	require.NoError(t, err)
	assert.Equal(t, uint32(123), response.GrantId)
	assert.Equal(t, uint32(456), response.CourseId)
}

func TestRedeemIntegrationAuthorizationRejectsMalformedValues(t *testing.T) {
	valid := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0}, 32))
	code := "685aebd3-45ba-4d4c-91a5-b181932f2ff5"
	tests := map[string]*protobuf.RedeemIntegrationAuthorizationRequest{
		"invalid alphabet": {Code: "%", State: valid},
		"wrong length":     {Code: code[:35], State: valid},
		"uppercase":        {Code: strings.ToUpper(code), State: valid},
		"braces":           {Code: "{" + code + "}", State: valid},
		"urn":              {Code: "urn:uuid:" + code, State: valid},
		"version 1":        {Code: "685aebd3-45ba-1d4c-91a5-b181932f2ff5", State: valid},
		"wrong variant":    {Code: "685aebd3-45ba-4d4c-c1a5-b181932f2ff5", State: valid},
		"old code":         {Code: valid, State: valid},
		"state padding":    {Code: code, State: valid + "="},
		"short state":      {Code: code, State: "short"},
		"state canonical":  {Code: code, State: valid[:len(valid)-1] + "B"},
	}
	for name, request := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: mock_dao.NewMockIntegrationGrantDao(ctrl)}, log: slog.Default()}

			_, err := api.RedeemIntegrationAuthorization(context.Background(), request)

			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestRedeemIntegrationAuthorizationReportsLookupFailures(t *testing.T) {
	request := &protobuf.RedeemIntegrationAuthorizationRequest{
		Code:  "685aebd3-45ba-4d4c-91a5-b181932f2ff5",
		State: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)),
	}
	for name, test := range map[string]struct {
		err  error
		code codes.Code
		msg  string
	}{
		"missing or unusable code": {gorm.ErrRecordNotFound, codes.NotFound, "authorization not found"},
		"database failure":         {errors.New("database unavailable"), codes.Unknown, "could not redeem authorization"},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
			integrationGrantDao.EXPECT().RedeemIntegrationAuthorizationCode(gomock.Any(), uint(7), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(0), uint(0), test.err)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

			_, err := api.RedeemIntegrationAuthorization(ctx, request)

			assert.Equal(t, test.code, status.Code(err))
			assert.Equal(t, test.msg, status.Convert(err).Message())
		})
	}
}

func TestGetIntegrationGrantMapsLiveCourseAndScopesLookup(t *testing.T) {
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	integrationGrantDao.EXPECT().GetIntegrationGrantCourse(gomock.Any(), uint(123), uint(7)).Return(model.Course{
		Model: gorm.Model{ID: 456}, Name: "Algorithms", Slug: "algo", Visibility: "hidden",
	}, nil)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

	response, err := api.GetIntegrationGrant(ctx, &protobuf.GetIntegrationGrantRequest{GrantId: 123})

	require.NoError(t, err)
	assert.Equal(t, &protobuf.GetIntegrationGrantResponse{
		CourseId: 456, Name: "Algorithms", Slug: "algo", Visibility: "hidden",
	}, response)
}

func TestGetIntegrationGrantRejectsZeroWithoutQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: mock_dao.NewMockIntegrationGrantDao(ctrl)}, log: slog.Default()}

	_, err := api.GetIntegrationGrant(context.Background(), &protobuf.GetIntegrationGrantRequest{})

	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetIntegrationGrantReportsIneligibleAndDatabaseFailures(t *testing.T) {
	for name, test := range map[string]struct {
		err  error
		code codes.Code
		msg  string
	}{
		"missing revoked or other application": {gorm.ErrRecordNotFound, codes.NotFound, "grant not found"},
		"database failure":                     {errors.New("database unavailable"), codes.Unknown, "could not read integration grant"},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
			integrationGrantDao.EXPECT().GetIntegrationGrantCourse(gomock.Any(), uint(123), uint(7)).Return(model.Course{}, test.err)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

			_, err := api.GetIntegrationGrant(ctx, &protobuf.GetIntegrationGrantRequest{GrantId: 123})

			assert.Equal(t, test.code, status.Code(err))
			assert.Equal(t, test.msg, status.Convert(err).Message())
		})
	}
}

func TestRevokeIntegrationGrantScopesWriteAuditsOnceAndRepeatsSuccessfully(t *testing.T) {
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	auditDao := mock_dao.NewMockAuditDao(ctrl)
	course := model.Course{Model: gorm.Model{ID: 3}, Name: "Winter course", Slug: "winter"}
	gomock.InOrder(
		integrationGrantDao.EXPECT().RevokeIntegrationGrantForIntegration(gomock.Any(), uint(123), uint(7)).Return(course, nil),
		integrationGrantDao.EXPECT().RevokeIntegrationGrantForIntegration(gomock.Any(), uint(123), uint(7)).Return(model.Course{}, nil),
	)
	auditDao.EXPECT().Create(gomock.Any()).DoAndReturn(func(audit *model.Audit) error {
		assert.Nil(t, audit.User)
		assert.Equal(t, model.AuditCourseEdit, audit.Type)
		assert.Equal(t, "Winter course:'winter' revoke integration grant: 123 by integration 'Course portal' (7)", audit.Message)
		return nil
	})
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, AuditDao: auditDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7, Name: "Course portal"}})
	request := &protobuf.RevokeIntegrationGrantRequest{GrantId: 123}

	for range 2 {
		response, err := api.RevokeIntegrationGrant(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, &emptypb.Empty{}, response)
	}
}

func TestRevokeIntegrationGrantRejectsZeroWithoutWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: mock_dao.NewMockIntegrationGrantDao(ctrl)}, log: slog.Default()}

	_, err := api.RevokeIntegrationGrant(context.Background(), &protobuf.RevokeIntegrationGrantRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRevokeIntegrationGrantHidesDatabaseFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	integrationGrantDao.EXPECT().RevokeIntegrationGrantForIntegration(gomock.Any(), uint(123), uint(7)).Return(model.Course{}, errors.New("secret database detail"))
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

	_, err := api.RevokeIntegrationGrant(ctx, &protobuf.RevokeIntegrationGrantRequest{GrantId: 123})
	assert.Equal(t, codes.Unknown, status.Code(err))
	assert.Equal(t, "could not revoke integration grant", status.Convert(err).Message())
}

func TestRevokeIntegrationGrantPolicyIsIntegrationOnly(t *testing.T) {
	fullMethod := method(&protobuf.IntegrationService_ServiceDesc, "revokeIntegrationGrant")
	assert.Equal(t, integrationOnly, methodPolicies[fullMethod])
}

func stubIntegrationSigner(t *testing.T, err error) *[]model.Stream {
	t.Helper()
	var signed []model.Stream
	orig := signPlaylists
	signPlaylists = func(s *model.Stream, user *model.User, allowDownloading bool) error {
		assert.Nil(t, user, "integration playlists are signed without a user")
		assert.False(t, allowDownloading, "integration playlists allow no downloads")
		signed = append(signed, *s)
		if err != nil {
			return err
		}
		for _, playlist := range []*string{&s.PlaylistUrl, &s.PlaylistUrlPRES, &s.PlaylistUrlCAM} {
			if *playlist != "" {
				*playlist += "?jwt=signed"
			}
		}
		return nil
	}
	t.Cleanup(func() { signPlaylists = orig })
	return &signed
}

func TestGetIntegrationRecordingSignsOnlyTheCombinedVersionOutsideLectureHalls(t *testing.T) {
	signed := stubIntegrationSigner(t, nil)
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(model.Stream{
		Model: gorm.Model{ID: 55}, CourseID: 456, Recording: true,
		PlaylistUrl: "https://edge/comb.m3u8", PlaylistUrlCAM: "https://edge/cam.m3u8", PlaylistUrlPRES: "https://edge/pres.m3u8",
	}, nil)
	streamsDao.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(55)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, StreamsDao: streamsDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

	response, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55})

	require.NoError(t, err)
	assert.Equal(t, &protobuf.GetIntegrationRecordingResponse{PlaylistUrl: "https://edge/comb.m3u8?jwt=signed"}, response)
	require.Len(t, *signed, 1)
	assert.Equal(t, uint(55), (*signed)[0].ID)
	assert.Equal(t, uint(456), (*signed)[0].CourseID)
	assert.Empty(t, (*signed)[0].PlaylistUrlCAM)
	assert.Empty(t, (*signed)[0].PlaylistUrlPRES)
}

func TestGetIntegrationRecordingHandsOutTheVersionTheWatchPageStartsWith(t *testing.T) {
	all := model.Stream{PlaylistUrl: "https://edge/comb.m3u8", PlaylistUrlPRES: "https://edge/pres.m3u8", PlaylistUrlCAM: "https://edge/cam.m3u8"}
	for name, test := range map[string]struct {
		stream  model.Stream
		mode    model.SourceMode
		version model.StreamVersion
		want    string
	}{
		"hall recorded combined":           {stream: all, mode: model.SourceModeCOMB, version: model.COMB, want: "https://edge/comb.m3u8?jwt=signed"},
		"hall without combined playlist":   {stream: model.Stream{PlaylistUrlPRES: all.PlaylistUrlPRES, PlaylistUrlCAM: all.PlaylistUrlCAM}, mode: model.SourceModeCOMB, version: model.PRES, want: "https://edge/pres.m3u8?jwt=signed"},
		"hall recorded presentation only":  {stream: all, mode: model.SourceModePRESOnly, version: model.PRES, want: "https://edge/pres.m3u8?jwt=signed"},
		"hall recorded camera only":        {stream: all, mode: model.SourceModeCAMOnly, version: model.CAM, want: "https://edge/cam.m3u8?jwt=signed"},
		"camera only and not recorded yet": {mode: model.SourceModeCAMOnly, version: model.CAM},
	} {
		t.Run(name, func(t *testing.T) {
			signed := stubIntegrationSigner(t, nil)
			ctrl := gomock.NewController(t)
			integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
			streamsDao := mock_dao.NewMockStreamsDao(ctrl)
			coursesDao := mock_dao.NewMockCoursesDao(ctrl)
			stream := test.stream
			stream.Model, stream.CourseID, stream.LectureHallID, stream.Recording = gorm.Model{ID: 55}, 456, 3, test.want != ""
			course := model.Course{Model: gorm.Model{ID: 456}}
			course.SetSourcePreference([]model.SourcePreference{{LectureHallID: 3, SourceMode: test.mode}})
			integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(stream, nil)
			coursesDao.EXPECT().GetCourseById(gomock.Any(), uint(456)).Return(course, nil)
			streamsDao.EXPECT().GetTranscodingProgressByVersion(test.version, uint(55)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, StreamsDao: streamsDao, CoursesDao: coursesDao}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

			response, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55})

			require.NoError(t, err)
			assert.Equal(t, test.want, response.GetPlaylistUrl())
			if test.want != "" {
				// Only the handed-out version is signed.
				require.Len(t, *signed, 1)
				assert.Equal(t, test.want, *playlistOf(&(*signed)[0], test.version)+"?jwt=signed")
			} else {
				assert.Empty(t, *signed)
			}
		})
	}
}

func TestGetIntegrationRecordingHandsOutTheRequestedVersion(t *testing.T) {
	all := model.Stream{PlaylistUrl: "https://edge/comb.m3u8", PlaylistUrlPRES: "https://edge/pres.m3u8", PlaylistUrlCAM: "https://edge/cam.m3u8"}
	for name, test := range map[string]struct {
		stream    model.Stream
		mode      model.SourceMode
		requested protobuf.VideoType
		want      string
	}{
		"presentation of a hall recorded combined": {stream: all, mode: model.SourceModeCOMB, requested: protobuf.VideoType_PRES, want: "https://edge/pres.m3u8?jwt=signed"},
		"camera of a hall recorded combined":       {stream: all, mode: model.SourceModeCOMB, requested: protobuf.VideoType_CAM, want: "https://edge/cam.m3u8?jwt=signed"},
		"combined without a combined playlist":     {stream: model.Stream{PlaylistUrlPRES: all.PlaylistUrlPRES}, mode: model.SourceModeCOMB, requested: protobuf.VideoType_COMB},
		"camera the hall does not record":          {stream: all, mode: model.SourceModePRESOnly, requested: protobuf.VideoType_CAM},
	} {
		t.Run(name, func(t *testing.T) {
			signed := stubIntegrationSigner(t, nil)
			ctrl := gomock.NewController(t)
			integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
			streamsDao := mock_dao.NewMockStreamsDao(ctrl)
			coursesDao := mock_dao.NewMockCoursesDao(ctrl)
			stream := test.stream
			stream.Model, stream.CourseID, stream.LectureHallID, stream.Recording = gorm.Model{ID: 55}, 456, 3, true
			course := model.Course{Model: gorm.Model{ID: 456}}
			course.SetSourcePreference([]model.SourcePreference{{LectureHallID: 3, SourceMode: test.mode}})
			integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(stream, nil)
			coursesDao.EXPECT().GetCourseById(gomock.Any(), uint(456)).Return(course, nil)
			version := model.StreamVersion(test.requested.String())
			if test.want != "" {
				streamsDao.EXPECT().GetTranscodingProgressByVersion(version, uint(55)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
			}
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, StreamsDao: streamsDao, CoursesDao: coursesDao}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

			response, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55, StreamVersion: &test.requested})

			if test.want == "" {
				assert.Equal(t, codes.NotFound, status.Code(err))
				assert.Equal(t, "recording version not found", status.Convert(err).Message())
				assert.Empty(t, *signed)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, response.GetPlaylistUrl())
			require.Len(t, *signed, 1)
			assert.Equal(t, test.want, *playlistOf(&(*signed)[0], version)+"?jwt=signed")
		})
	}
}

func TestGetIntegrationRecordingHidesCourseLookupFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(model.Stream{Model: gorm.Model{ID: 55}, CourseID: 456, LectureHallID: 3}, nil)
	coursesDao.EXPECT().GetCourseById(gomock.Any(), uint(456)).Return(model.Course{}, errors.New("secret database detail"))
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, CoursesDao: coursesDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

	_, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55})

	assert.Equal(t, codes.Unknown, status.Code(err))
	assert.Equal(t, "could not read recording", status.Convert(err).Message())
}

func TestGetIntegrationRecordingReportsConversionState(t *testing.T) {
	for name, test := range map[string]struct {
		stream   model.Stream
		progress model.TranscodingProgress
		err      error
		want     *protobuf.GetIntegrationRecordingResponse
	}{
		"first upload converting": {
			stream:   model.Stream{Model: gorm.Model{ID: 55}},
			progress: model.TranscodingProgress{Progress: 40},
			want:     &protobuf.GetIntegrationRecordingResponse{Converting: true, Progress: 40},
		},
		"re-upload converting keeps the old playlist": {
			stream:   model.Stream{Model: gorm.Model{ID: 55}, Recording: true, PlaylistUrl: "https://edge/comb.m3u8"},
			progress: model.TranscodingProgress{Progress: 10},
			want:     &protobuf.GetIntegrationRecordingResponse{PlaylistUrl: "https://edge/comb.m3u8?jwt=signed", Converting: true, Progress: 10},
		},
		"failed": {
			stream:   model.Stream{Model: gorm.Model{ID: 55}},
			progress: model.TranscodingProgress{Progress: 70, Failed: true, Error: "The file is not a video."},
			want:     &protobuf.GetIntegrationRecordingResponse{Failed: true, Error: "The file is not a video."},
		},
		"planned lecture without a recording": {
			stream: model.Stream{Model: gorm.Model{ID: 55}},
			err:    gorm.ErrRecordNotFound,
			want:   &protobuf.GetIntegrationRecordingResponse{},
		},
		"live playlist kept while another version's VOD is ready first, as the watch page plays it": {
			stream: model.Stream{Model: gorm.Model{ID: 55}, Recording: true, PlaylistUrl: "https://edge/live.m3u8"},
			err:    gorm.ErrRecordNotFound,
			want:   &protobuf.GetIntegrationRecordingResponse{PlaylistUrl: "https://edge/live.m3u8?jwt=signed"},
		},
		"live playlist of a lecture that is not recorded yet": {
			stream: model.Stream{Model: gorm.Model{ID: 55}, LiveNow: true, PlaylistUrl: "https://edge/live.m3u8"},
			err:    gorm.ErrRecordNotFound,
			want:   &protobuf.GetIntegrationRecordingResponse{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			stubIntegrationSigner(t, nil)
			ctrl := gomock.NewController(t)
			integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
			streamsDao := mock_dao.NewMockStreamsDao(ctrl)
			integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(test.stream, nil)
			streamsDao.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(55)).Return(test.progress, test.err)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, StreamsDao: streamsDao}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

			response, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55})

			require.NoError(t, err)
			assert.Equal(t, test.want, response)
		})
	}
}

func TestGetIntegrationRecordingRejectsZeroWithoutQuery(t *testing.T) {
	for name, request := range map[string]*protobuf.GetIntegrationRecordingRequest{
		"no grant":  {StreamId: 55},
		"no stream": {GrantId: 123},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: mock_dao.NewMockIntegrationGrantDao(ctrl)}, log: slog.Default()}

			_, err := api.GetIntegrationRecording(context.Background(), request)

			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestGetIntegrationRecordingHidesFailures(t *testing.T) {
	recorded := model.Stream{Model: gorm.Model{ID: 55}, Recording: true, PlaylistUrl: "https://edge/comb.m3u8"}
	for name, test := range map[string]struct {
		lookupErr, progressErr, signErr error
		code                            codes.Code
		msg                             string
	}{
		"missing private revoked or other application": {lookupErr: gorm.ErrRecordNotFound, code: codes.NotFound, msg: "recording not found"},
		"lookup failure":   {lookupErr: errors.New("secret database detail"), code: codes.Unknown, msg: "could not read recording"},
		"progress failure": {progressErr: errors.New("secret database detail"), code: codes.Unknown, msg: "could not read recording"},
		"signing failure":  {progressErr: gorm.ErrRecordNotFound, signErr: errors.New("no key"), code: codes.Unknown, msg: "could not sign playlist"},
	} {
		t.Run(name, func(t *testing.T) {
			stubIntegrationSigner(t, test.signErr)
			ctrl := gomock.NewController(t)
			integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
			streamsDao := mock_dao.NewMockStreamsDao(ctrl)
			integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(recorded, test.lookupErr)
			if test.lookupErr == nil {
				streamsDao.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(55)).Return(model.TranscodingProgress{}, test.progressErr)
			}
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, StreamsDao: streamsDao}, log: slog.Default()}
			ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

			_, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55})

			assert.Equal(t, test.code, status.Code(err))
			assert.Equal(t, test.msg, status.Convert(err).Message())
		})
	}
}

func TestGetIntegrationRecordingReturnsLegacyLRZPlaylistAsTheWatchPagePlaysIt(t *testing.T) {
	// The real signer, which skips LRZ playlists before it needs a key.
	ctrl := gomock.NewController(t)
	integrationGrantDao := mock_dao.NewMockIntegrationGrantDao(ctrl)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	integrationGrantDao.EXPECT().GetIntegrationGrantStream(gomock.Any(), uint(123), uint(7), uint(55)).Return(model.Stream{
		Model: gorm.Model{ID: 55}, Recording: true, PlaylistUrl: "https://stream.lrz.de/vod/old.m3u8",
	}, nil)
	streamsDao.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(55)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: integrationGrantDao, StreamsDao: streamsDao}, log: slog.Default()}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7}})

	response, err := api.GetIntegrationRecording(ctx, &protobuf.GetIntegrationRecordingRequest{GrantId: 123, StreamId: 55})

	require.NoError(t, err)
	assert.Equal(t, "https://stream.lrz.de/vod/old.m3u8", response.PlaylistUrl)
}

func TestIsV4RFC4122(t *testing.T) {
	assert.True(t, isV4RFC4122(uuid.MustParse("685aebd3-45ba-4d4c-91a5-b181932f2ff5")))
	assert.False(t, isV4RFC4122(uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")), "v1")
	assert.False(t, isV4RFC4122(uuid.MustParse("018f3c1e-7b2a-7c3d-91a5-b181932f2ff5")), "v7")
	assert.False(t, isV4RFC4122(uuid.MustParse("685aebd3-45ba-4d4c-c1a5-b181932f2ff5")), "wrong variant")
}
