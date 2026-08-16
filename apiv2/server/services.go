package apiv2

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// service is how one of the API's services is served over gRPC, exposed over REST,
// and policed. Kept together so adding a service is one edit rather than three, the
// forgettable one being the policy.
type service struct {
	desc     *grpc.ServiceDesc
	register func(grpc.ServiceRegistrar, *API)
	gateway  func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
	policies map[string]accessPolicy
}

// services is the whole API. The order is the order the docs read in.
var services = []service{
	{
		desc:     &protobuf.MetaService_ServiceDesc,
		register: func(s grpc.ServiceRegistrar, a *API) { protobuf.RegisterMetaServiceServer(s, a) },
		gateway:  protobuf.RegisterMetaServiceHandlerFromEndpoint,
		policies: map[string]accessPolicy{
			"healthCheck":            public,
			"getSemesters":           public,
			"getServerNotifications": public,
			"getNotifications":       authenticated,
		},
	},
	{
		desc:     &protobuf.UserService_ServiceDesc,
		register: func(s grpc.ServiceRegistrar, a *API) { protobuf.RegisterUserServiceServer(s, a) },
		gateway:  protobuf.RegisterUserServiceHandlerFromEndpoint,
		policies: map[string]accessPolicy{
			// Drive the login page itself.
			"getLoginOptions": public,
			"resetPassword":   public,

			// The rest act on one particular account.
			"getUser":            authenticated,
			"updateUserSettings": authenticated,
			"exportPersonalData": authenticated,
		},
	},
	{
		desc:     &protobuf.CourseService_ServiceDesc,
		register: func(s grpc.ServiceRegistrar, a *API) { protobuf.RegisterCourseServiceServer(s, a) },
		gateway:  protobuf.RegisterCourseServiceHandlerFromEndpoint,
		policies: map[string]accessPolicy{
			// Browsing; the handlers filter by visibility.
			"getPublicCourses": public,
			"getCourseBySlug":  public,
			"getLiveCourses":   public,

			// Tied to one account's enrolments and pins.
			"getUserCourses":   authenticated,
			"getPinnedCourses": authenticated,
			"getPinForCourse":  authenticated,
			"pinCourse":        authenticated,
		},
	},
	{
		desc:     &protobuf.StreamService_ServiceDesc,
		register: func(s grpc.ServiceRegistrar, a *API) { protobuf.RegisterStreamServiceServer(s, a) },
		gateway:  protobuf.RegisterStreamServiceHandlerFromEndpoint,
		policies: map[string]accessPolicy{
			// Watching; authorizeUserForStreamCourse checks the course.
			"getStream":         public,
			"getVideoSections":  public,
			"getStreamPlaylist": public,
			"getSubtitles":      public,
			"getThumbs":         public,

			// One account's progress and bookmarks.
			"getProgressBatch": authenticated,
			"updateProgress":   authenticated,
			"addBookmark":      authenticated,
			"getBookmarks":     authenticated,
			"updateBookmark":   authenticated,
			"deleteBookmark":   authenticated,
		},
	},
}
