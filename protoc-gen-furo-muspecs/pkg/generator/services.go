package generator

import (
	"github.com/eclipse/eclipsefuro/furo/pkg/microservices"
	"github.com/eclipse/eclipsefuro/protoc-gen-furo-muspecs/pkg/protoast"
	options "google.golang.org/genproto/googleapis/api/annotations"
	"strings"
)

func getServices(serviceInfo protoast.ServiceInfo, sourceInfo protoast.SourceInfo) []microservices.MicroRPC {

	methodlist := []microservices.MicroRPC{}
	for _, methodInfo := range serviceInfo.Methods {

		rpcMethodDescription := ""
		if methodInfo.Info.LeadingComments != nil {
			rpcMethodDescription = cleanDescription(*methodInfo.Info.LeadingComments)
		}

		// if methodInfo.HttpRule.Info != nil && methodInfo.HttpRule.Info.LeadingComments != nil {
		//   deeplinkDescription = (*methodInfo.HttpRule.Info.LeadingComments)
		// }

		// extract api options to href method and rel

		// *methodInfo.HttpRule.ApiOptions.Pattern is oneof
		// details: vendor/google.golang.org/genproto/googleapis/api/annotations/http.pb.go:400

		href := ""
		verb := ""
		if methodInfo.HttpRule.ApiOptions != nil {
			href, verb, _, _ = extractApiOptionPattern(methodInfo.HttpRule)
		}
		// request type
		req := *methodInfo.Method.InputType
		inputType := req[1:len(req)]
		// response type
		res := *methodInfo.Method.OutputType
		outputType := res[1:len(res)]

		mRpc := microservices.MicroRPC{}

		mdLine := []string{}
		mdLine = append(mdLine, *methodInfo.Method.Name+":")
		mdLine = append(mdLine, verb)
		mdLine = append(mdLine, href)
		if inputType == "" {
			inputType = "google.protobuf.Empty"
		}
		mdLine = append(mdLine, inputType)
		mdLine = append(mdLine, ",")

		mdLine = append(mdLine, outputType)
		mdLine = append(mdLine, "#"+rpcMethodDescription)

		mRpc.Md = strings.Join(mdLine, " ") //List: GET /samples google.protobuf.Empty, sample.SampleCollection #List samples with pagination.

		methodlist = append(methodlist, mRpc)

	}

	return methodlist
}

// get the href, method, rel
func extractApiOptionPattern(info *protoast.ApiOptionInfo) (href string, method string, rel string, body string) {

	pattern := info.ApiOptions.Pattern
	href = "/no/option/given"
	method = "GET"
	rel = "self"
	body = info.ApiOptions.Body

	if info.Info != nil {
		// try first line of comment for the rel
		//   Delete: DELETE /samples/{xxx} google.protobuf.Empty, google.protobuf.Empty #Use this to delete existing samples.
		// becomes delete
		if info.Info.LeadingComments != nil {
			c := strings.Split(*info.Info.LeadingComments, ":")
			if len(c) > 1 && len(strings.TrimSpace(c[0])) > 3 {
				rel = strings.ToLower(strings.TrimSpace(c[0]))
			}

		}

	}

	get, isGet := pattern.(*options.HttpRule_Get)
	if isGet {
		href = get.Get
		method = "GET"
		if rel == "" {
			rel = "self"
		}
		return href, method, rel, body
	}

	post, isPost := pattern.(*options.HttpRule_Post)
	if isPost {
		href = post.Post
		method = "POST"
		if rel == "" {
			rel = "create"
		}
		return href, method, rel, body
	}

	patch, isPatch := pattern.(*options.HttpRule_Patch)
	if isPatch {
		href = patch.Patch
		method = "PATCH"
		if rel == "" {
			rel = "update"
		}
		return href, method, rel, body
	}

	put, isPut := pattern.(*options.HttpRule_Put)
	if isPut {
		href = put.Put
		method = "PUT"
		if rel == "" {
			rel = "update"
		}
		return href, method, rel, body
	}

	delete, isDelete := pattern.(*options.HttpRule_Delete)
	if isDelete {
		href = delete.Delete
		method = "DELETE"
		if rel == "" {
			rel = "delete"
		}
		return href, method, rel, body
	}

	// custom is for the verb and not for the custom method...
	custom, isCustom := pattern.(*options.HttpRule_Custom)
	if isCustom {
		href = custom.Custom.Path
		method = custom.Custom.Kind
		rel = strings.ToLower(method)
		return href, method, rel, body
	}

	return href, method, rel, body
}
