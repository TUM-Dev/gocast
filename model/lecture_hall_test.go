package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBeforeSaveLectureHall(t *testing.T) {
	cases := []struct {
		l        LectureHall
		expected LectureHall
		wantErr  bool
	}{
		{
			l: LectureHall{
				CameraIP: "not an ip",
			},
			wantErr: true,
		},
		{
			l: LectureHall{
				CameraIP: "127.0.0.almostanip",
			},
			expected: LectureHall{
				CameraIP: "",
			},
			wantErr: true,
		},
		{
			l: LectureHall{
				CameraIP: "127.0.0.1",
				CamIP:    "somehost/malicious\" && stuff",
			},
			expected: LectureHall{
				CameraIP: "127.0.0.1",
				CamIP:    "somehost/malicious%22%20&&%20stuff",
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("BeforeSave(%+v)", tc.l), func(t *testing.T) {
			err := tc.l.BeforeSave(nil)
			if err != nil && !tc.wantErr {
				t.Errorf("BeforeCreateLectureHall(%+v): got error %v, want no error", tc.l, err)
			} else if err == nil && tc.wantErr {
				t.Errorf("BeforeCreateLectureHall(%+v): got no error, want error", tc.l)
			}
			if err == nil {
				assert.Equal(t, tc.expected, tc.l)
			}
		})
	}
}
