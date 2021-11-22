# Techstack

TUM-Live is primarily developed in Go. We mainly use these frameworks and libraries:
- Gin (our HTTP router)
- Gorm (our database library)
- template/html (for backend HTML rendering)

Our frontend is mainly "from scratch" HTML rendered in the backend. We also use some
- aplinejs (for a little more dynamic pages)
- videojs (as our video player)
- tailwindcss (for styling)
- typescript/javascript

We have a decently large fleat of machines commuinicating with each other.
- gRPC is used for remote procedure calls
- Docker and Ansible are used for deployments, we want do do a little more with them and perhaps Kubernetes in the future

The actual streaming involves
- ffmpeg
- Wowza streaming engine
- nginx + nginx-rtmp-module
- We currently consider OvenMediaEngine for future streams


Working with us doesn't require you to be an expert in any of these. Developing software is a learning by doing job :)
