package xbox

import "net/http"

func doXboxRequest(req *http.Request) (*http.Response, error) { return httpClient.Do(req) }
