# CAMPUSonline API Client

This package contains Go clients for the CAMPUSonline REST APIs used by TUM-Live.

## APIs

| API | Spec | Package |
|-----|------|---------|
| Course API | `spec/course.json` | `gen/course` |
| Account API | `spec/account.json` | `gen/account` |

The OpenAPI specs are sourced from the CAMPUSonline public API portal:
**https://review.campus.tum.de/RSYSTEM/co/public**

## Regenerating the clients

The clients in `gen/` are generated from the OpenAPI specs in `spec/` using
[OpenAPI Generator](https://openapi-generator.tech).

### Prerequisites

Install the OpenAPI Generator CLI. The easiest way is via the published JAR:

```bash
# requires Java 11+
curl -L https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/7.6.0/openapi-generator-cli-7.6.0.jar \
  -o openapi-generator-cli.jar
```

Or via Homebrew:

```bash
brew install openapi-generator
```

### Generate the course client

```bash
openapi-generator generate \
  -i spec/course.json \
  -g go \
  -o gen/course \
  --package-name campusonline \
  --additional-properties=enumClassPrefix=true
```

### Generate the account client

```bash
openapi-generator generate \
  -i spec/account.json \
  -g go \
  -o gen/account \
  --package-name campusonline \
  --additional-properties=enumClassPrefix=true
```

### Updating the specs

Download fresh specs from the CAMPUSonline public portal:

```bash
curl -o spec/course.json  https://review.campus.tum.de/RSYSTEM/co/public/<course-spec-path>
curl -o spec/account.json https://review.campus.tum.de/RSYSTEM/co/public/<account-spec-path>
```

Browse the available APIs and their spec download URLs at:
**https://review.campus.tum.de/RSYSTEM/co/public**

After updating a spec, re-run the corresponding generation command above and
review any breaking changes in `campusonline.go`.
