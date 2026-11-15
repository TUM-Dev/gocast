package tools

type RequestError struct {
	Status        int
	CustomMessage string
	Err           error
}

func (r RequestError) Error() string {
	if r.Err != nil {
		return r.Err.Error()
	}
	return ""
}

func (r RequestError) ToResponse() map[string]any {
	res := map[string]any{"status": r.Status, "message": r.CustomMessage}

	if r.Err != nil {
		res["error"] = r.Error()
	}

	return res
}
