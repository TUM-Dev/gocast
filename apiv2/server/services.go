package apiv2

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
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
			"getFrontendConfig":      public,
			"getSemesters":           public,
			"getServerNotifications": public,
			"getInfoPage":            public,
			"listInfoPages":          public,
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

			// The course's own statistics, for its lecturers.
			"getCourseStats":    requiresCourseAdmin(),
			"exportCourseStats": requiresCourseAdmin(),
			// The handler also checks the lecture is the course's.
			"getLectureStats": requiresCourseAdmin(),

			// Any lecturer may start a course; the creator becomes its administrator.
			"createCourse":           requires(model.PermLecture),
			"searchTumOnlineCourses": requires(model.PermLecture),

			// The schedule filters to the caller's own courses itself; the hall names
			// are what the template showed every lecturer.
			"getSchedule":              requires(model.PermLecture),
			"listScheduleLectureHalls": requires(model.PermLecture),
			// The handler also checks the lecture is the course's.
			"updateLecture": requiresCourseAdmin(),

			// Lecture management. Each handler also checks the lecture is the
			// course's; copyLecture checks the caller administers the target too.
			"listCourseLecturesAdmin": requiresCourseAdmin(),
			"updateLectureSeries":     requiresCourseAdmin(),
			"updateLectureSeriesTime": requiresCourseAdmin(),
			"deleteLectures":          requiresCourseAdmin(),
			"deleteLectureSeries":     requiresCourseAdmin(),
			"copyLecture":             requiresCourseAdmin(),

			// ----- Course administration -----
			// The course page, for its administrators. The handlers also refuse
			// course 0, which the statistics queries read as every course.
			"getCourseAdmin":                  requiresCourseAdmin(),
			"updateCourseSettings":            requiresCourseAdmin(),
			"copyCourse":                      requiresCourseAdmin(),
			"deleteCourse":                    requiresCourseAdmin(),
			"listCourseAdmins":                requiresCourseAdmin(),
			"addCourseAdmin":                  requiresCourseAdmin(),
			"removeCourseAdmin":               requiresCourseAdmin(),
			"listCourseLectureHallSettings":   requiresCourseAdmin(),
			"updateCourseLectureHallSettings": requiresCourseAdmin(),
			"listCourseParticipants":          requiresCourseAdmin(),
			"inviteCourseParticipants":        requiresCourseAdmin(),
			// Course-scoped, unlike searchUsers: a course's lecturers lack
			// users.manage, and this answers only names, logins and roles.
			"searchUsersForCourse": requiresCourseAdmin(),
			// The caller's own courses, for the sidebar; any lecturer.
			"listAdministeredCourses": requires(model.PermLecture),

			// A lecture's content. Each handler also checks the lecture is the
			// course's, and a section or file named is the lecture's. The uploads beside
			// the gateway (lecture_upload.go) are authorized the same way in code.
			"createLectureSections":         requiresCourseAdmin(),
			"updateLectureSection":          requiresCourseAdmin(),
			"deleteLectureSection":          requiresCourseAdmin(),
			"deleteLectureAttachment":       requiresCourseAdmin(),
			"deleteLectureThumbnail":        requiresCourseAdmin(),
			"requestLectureSubtitles":       requiresCourseAdmin(),
			"getLectureTranscodingProgress": requiresCourseAdmin(),
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

			// Moves a lecture hall's camera; the handler checks the stream is the
			// course's and takes the hall from it.
			"switchCameraPreset": requiresCourseAdmin(),
		},
	},
	{
		desc:     &protobuf.AdminService_ServiceDesc,
		register: func(s grpc.ServiceRegistrar, a *API) { protobuf.RegisterAdminServiceServer(s, a) },
		gateway:  protobuf.RegisterAdminServiceHandlerFromEndpoint,
		policies: map[string]accessPolicy{
			// Runners belong to no course, so administering them is the server-wide
			// permission and nothing narrower. The handlers add no check of their own.
			"listRunners":  requires(model.PermAdministerServer),
			"deleteRunner": requires(model.PermAdministerServer),

			// Workers are the older, pre-runner equivalent; same permission.
			"listWorkers":  requires(model.PermAdministerServer),
			"deleteWorker": requires(model.PermAdministerServer),

			// Accounts are a different permission from the rest of the service. Both
			// belong to admins today; the split is what makes an operator role a
			// change to the role table rather than to every call site.
			"listStaff":      requires(model.PermManageUsers),
			"searchUsers":    requires(model.PermManageUsers),
			"createUser":     requires(model.PermManageUsers),
			"updateUserRole": requires(model.PermManageUsers),
			"deleteUser":     requires(model.PermManageUsers),

			// Info pages belong to no course, same as runners.
			"listInfoPagesAdmin": requires(model.PermAdministerServer),
			"createInfoPage":     requires(model.PermAdministerServer),
			"updateInfoPage":     requires(model.PermAdministerServer),
			"deleteInfoPage":     requires(model.PermAdministerServer),

			"listIntegrations":     requires(model.PermAdministerServer),
			"createIntegration":    requires(model.PermAdministerServer),
			"rotateIntegrationKey": requires(model.PermAdministerServer),
			"revokeIntegrationKey": requires(model.PermAdministerServer),

			// Maintenance operations belong to no course, same as runners.
			"getMaintenanceThumbnailStatus":       requires(model.PermAdministerServer),
			"generateMaintenanceThumbnails":       requires(model.PermAdministerServer),
			"listMaintenanceCronJobs":             requires(model.PermAdministerServer),
			"runMaintenanceCronJob":               requires(model.PermAdministerServer),
			"listMaintenanceTranscodingFailures":  requires(model.PermAdministerServer),
			"deleteMaintenanceTranscodingFailure": requires(model.PermAdministerServer),
			"listMaintenanceEmailFailures":        requires(model.PermAdministerServer),
			"deleteMaintenanceEmailFailure":       requires(model.PermAdministerServer),

			// Course import reaches TUMonline directly and creates courses outright;
			// server-wide, same as runners and info pages.
			"searchCourseImportSchedule": requires(model.PermAdministerServer),
			"importCourseImportCourses":  requires(model.PermAdministerServer),
			// Tokens are the only way to reach the API as another account, so they
			// stay behind the same permission as the accounts they authenticate as.
			"listTokens":  requires(model.PermManageUsers),
			"createToken": requires(model.PermManageUsers),
			"deleteToken": requires(model.PermManageUsers),
			// Server notifications belong to no course, same as runners.
			"listServerNotificationsAdmin": requires(model.PermAdministerServer),
			"createServerNotification":     requires(model.PermAdministerServer),
			"updateServerNotification":     requires(model.PermAdministerServer),
			"deleteServerNotification":     requires(model.PermAdministerServer),
			// Notifications belong to no course, same as runners and info pages.
			"listNotificationsAdmin": requires(model.PermAdministerServer),
			"createNotification":     requires(model.PermAdministerServer),
			"deleteNotification":     requires(model.PermAdministerServer),
			// The audit log spans every course and the server itself, so it takes the
			// same permission as runners and info pages rather than a course-scoped one.
			"listAudits": requires(model.PermAdministerServer),
			// Server-wide statistics, same permission the old /admin/server-stats
			// route required.
			"getServerStats":    requires(model.PermAdministerServer),
			"exportServerStats": requires(model.PermAdministerServer),
			// Lecture halls belong to no course, same as runners and info pages.
			"listLectureHallsAdmin":          requires(model.PermAdministerServer),
			"createLectureHallAdmin":         requires(model.PermAdministerServer),
			"updateLectureHallAdmin":         requires(model.PermAdministerServer),
			"deleteLectureHallAdmin":         requires(model.PermAdministerServer),
			"refreshLectureHallPresetsAdmin": requires(model.PermAdministerServer),
			"setDefaultCameraPresetAdmin":    requires(model.PermAdministerServer),
			"takeCameraPresetSnapshotAdmin":  requires(model.PermAdministerServer),
		},
	},
}
