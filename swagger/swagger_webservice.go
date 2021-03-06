package swagger

import (
	"fmt"

	"github.com/r4chi7/go-restful"
	// "gitlab.com/ridely/hopwatch"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/r4chi7/go-restful/log"
)

type SwaggerService struct {
	config            Config
	apiDeclarationMap *ApiDeclarationList
}

func newSwaggerService(config Config) *SwaggerService {
	return &SwaggerService{
		config:            config,
		apiDeclarationMap: new(ApiDeclarationList)}
}

// LogInfo is the function that is called when this package needs to log. It defaults to log.Printf
var LogInfo = func(format string, v ...interface{}) {
	// use the restful package-wide logger
	log.Printf(format, v...)
}

// InstallSwaggerService add the WebService that provides the API documentation of all services
// conform the Swagger documentation specifcation. (https://github.com/wordnik/swagger-core/wiki).
func InstallSwaggerService(aSwaggerConfig Config) {
	RegisterSwaggerService(aSwaggerConfig, restful.DefaultContainer)
}

// RegisterSwaggerService add the WebService that provides the API documentation of all services
// conform the Swagger documentation specifcation. (https://github.com/wordnik/swagger-core/wiki).
func RegisterSwaggerService(config Config, wsContainer *restful.Container) {
	sws := newSwaggerService(config)
	ws := new(restful.WebService)
	ws.Path(config.ApiPath)
	ws.Produces(restful.MIME_JSON)
	if config.DisableCORS {
		ws.Filter(enableCORS)
	}
	ws.Route(ws.GET("/").To(sws.getListing))
	ws.Route(ws.GET("/{a}").To(sws.getDeclarations))
	ws.Route(ws.GET("/{a}/{b}").To(sws.getDeclarations))
	ws.Route(ws.GET("/{a}/{b}/{c}").To(sws.getDeclarations))
	ws.Route(ws.GET("/{a}/{b}/{c}/{d}").To(sws.getDeclarations))
	ws.Route(ws.GET("/{a}/{b}/{c}/{d}/{e}").To(sws.getDeclarations))
	ws.Route(ws.GET("/{a}/{b}/{c}/{d}/{e}/{f}").To(sws.getDeclarations))
	ws.Route(ws.GET("/{a}/{b}/{c}/{d}/{e}/{f}/{g}").To(sws.getDeclarations))
	LogInfo("[restful/swagger] listing is available at %v%v", config.WebServicesUrl, config.ApiPath)
	wsContainer.Add(ws)

	// Build all ApiDeclarations
	for _, each := range config.WebServices {
		rootPath := each.RootPath()
		// skip the api service itself
		if rootPath != config.ApiPath {
			if rootPath == "" || rootPath == "/" {
				// use routes
				for _, route := range each.Routes() {
					entry := staticPathFromRoute(route)
					_, exists := sws.apiDeclarationMap.At(entry)
					if !exists {
						sws.apiDeclarationMap.Put(entry, sws.composeDeclaration(each, entry))
					}
				}
			} else { // use root path
				sws.apiDeclarationMap.Put(each.RootPath(), sws.composeDeclaration(each, each.RootPath()))
			}
		}
	}

	// if specified then call the PostBuilderHandler
	if config.PostBuildHandler != nil {
		config.PostBuildHandler(sws.apiDeclarationMap)
	}

	// Check paths for UI serving
	if config.StaticHandler == nil && config.SwaggerFilePath != "" && config.SwaggerPath != "" {
		swaggerPathSlash := config.SwaggerPath
		// path must end with slash /
		if "/" != config.SwaggerPath[len(config.SwaggerPath)-1:] {
			LogInfo("[restful/swagger] use corrected SwaggerPath ; must end with slash (/)")
			swaggerPathSlash += "/"
		}

		LogInfo("[restful/swagger] %v%v is mapped to folder %v", config.WebServicesUrl, swaggerPathSlash, config.SwaggerFilePath)
		wsContainer.Handle(swaggerPathSlash, http.StripPrefix(swaggerPathSlash, http.FileServer(http.Dir(config.SwaggerFilePath))))

		//if we define a custom static handler use it
	} else if config.StaticHandler != nil && config.SwaggerPath != "" {
		swaggerPathSlash := config.SwaggerPath
		// path must end with slash /
		if "/" != config.SwaggerPath[len(config.SwaggerPath)-1:] {
			LogInfo("[restful/swagger] use corrected SwaggerFilePath ; must end with slash (/)")
			swaggerPathSlash += "/"

		}
		LogInfo("[restful/swagger] %v%v is mapped to custom Handler %T", config.WebServicesUrl, swaggerPathSlash, config.StaticHandler)
		wsContainer.Handle(swaggerPathSlash, config.StaticHandler)

	} else {
		LogInfo("[restful/swagger] Swagger(File)Path is empty ; no UI is served")
	}
}

func staticPathFromRoute(r restful.Route) string {
	static := r.Path
	bracket := strings.Index(static, "{")
	if bracket <= 1 { // result cannot be empty
		return static
	}
	if bracket != -1 {
		static = r.Path[:bracket]
	}
	if strings.HasSuffix(static, "/") {
		return static[:len(static)-1]
	} else {
		return static
	}
}

func enableCORS(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if origin := req.HeaderParameter(restful.HEADER_Origin); origin != "" {
		// prevent duplicate header
		if len(resp.Header().Get(restful.HEADER_AccessControlAllowOrigin)) == 0 {
			resp.AddHeader(restful.HEADER_AccessControlAllowOrigin, origin)
		}
	}
	chain.ProcessFilter(req, resp)
}

func (sws SwaggerService) getListing(req *restful.Request, resp *restful.Response) {
	listing := ResourceListing{SwaggerVersion: swaggerVersion, ApiVersion: sws.config.ApiVersion}
	sws.apiDeclarationMap.Do(func(k string, v ApiDeclaration) {
		ref := Resource{Path: k}
		if len(v.Apis) > 0 { // use description of first (could still be empty)
			ref.Description = v.Apis[0].Description
		}
		listing.Apis = append(listing.Apis, ref)
	})
	resp.WriteAsJson(listing)
}

func (sws SwaggerService) getDeclarations(req *restful.Request, resp *restful.Response) {
	decl, ok := sws.apiDeclarationMap.At(composeRootPath(req))
	if !ok {
		resp.WriteErrorString(http.StatusNotFound, "ApiDeclaration not found")
		return
	}
	// unless WebServicesUrl is given
	if len(sws.config.WebServicesUrl) == 0 {
		// update base path from the actual request
		// TODO how to detect https? assume http for now
		var host string
		// X-Forwarded-Host or Host or Request.Host
		hostvalues, ok := req.Request.Header["X-Forwarded-Host"] // apache specific?
		if !ok || len(hostvalues) == 0 {
			forwarded, ok := req.Request.Header["Host"] // without reverse-proxy
			if !ok || len(forwarded) == 0 {
				// fallback to Host field
				host = req.Request.Host
			} else {
				host = forwarded[0]
			}
		} else {
			host = hostvalues[0]
		}
		(&decl).BasePath = fmt.Sprintf("http://%s", host)
	}
	resp.WriteAsJson(decl)
}

// composeDeclaration uses all routes and parameters to create a ApiDeclaration
func (sws SwaggerService) composeDeclaration(ws *restful.WebService, pathPrefix string) ApiDeclaration {
	decl := ApiDeclaration{
		SwaggerVersion: swaggerVersion,
		BasePath:       sws.config.WebServicesUrl,
		ResourcePath:   pathPrefix,
		Models:         ModelList{},
		ApiVersion:     ws.Version()}

	// collect any path parameters
	rootParams := []Parameter{}
	for _, param := range ws.PathParameters() {
		rootParams = append(rootParams, asSwaggerParameter(param.Data()))
	}
	// aggregate by path
	pathToRoutes := newOrderedRouteMap()
	for _, other := range ws.Routes() {
		if strings.HasPrefix(other.Path, pathPrefix) {
			pathToRoutes.Add(other.Path, other)
		}
	}
	pathToRoutes.Do(func(path string, routes []restful.Route) {
		api := Api{Path: strings.TrimSuffix(path, "/"), Description: ws.Documentation()}
		for _, route := range routes {
			operation := Operation{
				Method:           route.Method,
				Summary:          route.Doc,
				Notes:            route.Notes,
				Type:             asDataType(route.WriteSample),
				Parameters:       []Parameter{},
				Nickname:         route.Operation,
				ResponseMessages: composeResponseMessages(route, &decl)}

			operation.Consumes = route.Consumes
			operation.Produces = route.Produces

			// share root params if any
			for _, swparam := range rootParams {
				operation.Parameters = append(operation.Parameters, swparam)
			}
			// route specific params
			for _, param := range route.ParameterDocs {
				operation.Parameters = append(operation.Parameters, asSwaggerParameter(param.Data()))
			}

			sws.addModelsFromRouteTo(&operation, route, &decl)
			api.Operations = append(api.Operations, operation)
		}
		decl.Apis = append(decl.Apis, api)
	})
	return decl
}

// composeResponseMessages takes the ResponseErrors (if any) and creates ResponseMessages from them.
func composeResponseMessages(route restful.Route, decl *ApiDeclaration) (messages []ResponseMessage) {
	if route.ResponseErrors == nil {
		return messages
	}
	// sort by code
	codes := sort.IntSlice{}
	for code, _ := range route.ResponseErrors {
		codes = append(codes, code)
	}
	codes.Sort()
	for _, code := range codes {
		each := route.ResponseErrors[code]
		message := ResponseMessage{
			Code:    code,
			Message: each.Message,
		}
		if each.Model != nil {
			st := reflect.TypeOf(each.Model)
			isCollection, st := detectCollectionType(st)
			modelName := modelBuilder{}.keyFrom(st)
			if isCollection {
				modelName = "array[" + modelName + "]"
			}
			modelBuilder{&decl.Models}.addModel(st, "")
			// reference the model
			message.ResponseModel = modelName
		}
		messages = append(messages, message)
	}
	return
}

// addModelsFromRoute takes any read or write sample from the Route and creates a Swagger model from it.
func (sws SwaggerService) addModelsFromRouteTo(operation *Operation, route restful.Route, decl *ApiDeclaration) {
	if route.ReadSample != nil {
		sws.addModelFromSampleTo(operation, false, route.ReadSample, &decl.Models)
	}
	if route.WriteSample != nil {
		sws.addModelFromSampleTo(operation, true, route.WriteSample, &decl.Models)
	}
}

func detectCollectionType(st reflect.Type) (bool, reflect.Type) {
	isCollection := false
	if st.Kind() == reflect.Slice || st.Kind() == reflect.Array {
		st = st.Elem()
		isCollection = true
	} else {
		if st.Kind() == reflect.Ptr {
			if st.Elem().Kind() == reflect.Slice || st.Elem().Kind() == reflect.Array {
				st = st.Elem().Elem()
				isCollection = true
			}
		}
	}
	return isCollection, st
}

// addModelFromSample creates and adds (or overwrites) a Model from a sample resource
func (sws SwaggerService) addModelFromSampleTo(operation *Operation, isResponse bool, sample interface{}, models *ModelList) {
	st := reflect.TypeOf(sample)
	isCollection, st := detectCollectionType(st)
	modelName := modelBuilder{}.keyFrom(st)
	if isResponse {
		if isCollection {
			modelName = "array[" + modelName + "]"
		}
		operation.Type = modelName
	}
	modelBuilder{models}.addModelFrom(sample)
}

func asSwaggerParameter(param restful.ParameterData) Parameter {
	return Parameter{
		DataTypeFields: DataTypeFields{
			Type:         &param.DataType,
			Format:       asFormat(param.DataType),
			DefaultValue: Special(param.DefaultValue),
		},
		Name:        param.Name,
		Description: param.Description,
		ParamType:   asParamType(param.Kind),

		Required: param.Required}
}

// Between 1..7 path parameters is supported
func composeRootPath(req *restful.Request) string {
	path := "/" + req.PathParameter("a")
	b := req.PathParameter("b")
	if b == "" {
		return path
	}
	path = path + "/" + b
	c := req.PathParameter("c")
	if c == "" {
		return path
	}
	path = path + "/" + c
	d := req.PathParameter("d")
	if d == "" {
		return path
	}
	path = path + "/" + d
	e := req.PathParameter("e")
	if e == "" {
		return path
	}
	path = path + "/" + e
	f := req.PathParameter("f")
	if f == "" {
		return path
	}
	path = path + "/" + f
	g := req.PathParameter("g")
	if g == "" {
		return path
	}
	return path + "/" + g
}

func asFormat(name string) string {
	return "" // TODO
}

func asParamType(kind int) string {
	switch {
	case kind == restful.PathParameterKind:
		return "path"
	case kind == restful.QueryParameterKind:
		return "query"
	case kind == restful.BodyParameterKind:
		return "body"
	case kind == restful.HeaderParameterKind:
		return "header"
	case kind == restful.FormParameterKind:
		return "form"
	}
	return ""
}

func asDataType(any interface{}) string {
	if any == nil {
		return "void"
	}
	return reflect.TypeOf(any).Name()
}
