package ast

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"math"

	"sessions/ast/util"
	"sessions/ast/sysio"
	"sessions/ast/api"
)

/**********************************************************************************
 * Registries for handleExecutors
 **********************************************************************************/

/******** handle executor interfaces ********/

type inputHandleExecutor interface {
	execute(*recordExpr, typedef) (expression, error)
}

type outputHandleExecutor interface {
	execute(*recordExpr) (expression, error)
}

/******** registries for input and output handleExecutors ********/

type actionKey struct{ srv, ac string }

//TODO give a format for each executor
var inputExecutors = map[actionKey]inputHandleExecutor {
	actionKey{"io", "open"}: openFileExecutor{},
	actionKey{"io", "mqtt"}: mqttSubscriberExecutor{},
	actionKey{"memory", "read"}: memoryReaderExecutor{},
	actionKey{"json", "marshal"}: marshalExecutor{},
	actionKey{"json", "unmarshal"}: unmarshalExecutor{},
	actionKey{"time", "current"}: timeExecutor{},
	actionKey{"random", "generator"}: randomGeneratorExecutor{}, 

	actionKey{"runtime", "compile"}: compileExecutor{},

	actionKey{"api", "connect"}: connectAPIExecutor{},
	actionKey{"api", "request"}: requestAPIExecutor{},
}

var outputExecutors = map[actionKey]outputHandleExecutor{
	actionKey{"io", "mqtt"}: mqttPublisherExecutor{},
	actionKey{"memory", "write"}: memoryWriterExecutor{},
	actionKey{"time", "sleep"}: sleepExecutor{},
}

func findInputExecutor(srv, ac string) (inputHandleExecutor, bool) {
	c, ok := inputExecutors[actionKey{srv, ac}]
	return c, ok
}

func findOutputExecutor(srv, ac string) (outputHandleExecutor, bool) {
	c, ok := outputExecutors[actionKey{srv, ac}]
	return c, ok
}

/**********************************************************************************
 * handleExecutors
 **********************************************************************************/

 /******* server executor *********/

 type connectAPIExecutor struct{}

 func (connectAPIExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
 	name, ok := conf.readString("name")
 	if !ok {
 		return nil, fmt.Errorf("api: missing api name")
 	}

 	api, err := api.GetAPI(name)
 	if err != nil {
 		return nil, err
 	}
 	aHandle := &apiHandle {
 		emptyHandle:	emptyHandle{baseNode: baseNode{line: conf.line}},
 		api: api,
 		request: "",
 	}
 	return aHandle, nil
 }

type requestAPIExecutor struct{}

 func (requestAPIExecutor) execute(conf *recordExpr, tdef typedef) (expression, error) {
 	expr, ok1 := conf.exprMap["api"]
 	var err1 error
 	if !ok1 {
 		err1 = fmt.Errorf("api: missing api")
 	}

 	request, ok2 := conf.exprMap["request"]
 	var err2 error
 	if !ok2 {
 		err2 = fmt.Errorf("api: missing request")
 	}

 	value, ok3 := request.(valueI)
 	var err3 error
 	if !ok3 {
 		err3 = fmt.Errorf("api: request is not a value")
 	}

 	if !(ok1 && ok2 && ok3) {
		return nil, util.JoinSep("; ", err1, err2, err3)
	}

 	aHandle, ok := expr.(*apiHandle)
 	if !ok {
 		return nil, fmt.Errorf("api: expected api value")
 	}

 	aHandle.request = value.marshal()
 	return aHandle.input(tdef)
 }

/******** marshal executor ********/

type marshalExecutor struct{}

func (marshalExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	expr, ok :=  conf.exprMap["value"]
	if !ok {
		return nil, fmt.Errorf("json: missing value")
	}

	value, ok := expr.(valueI)
	if !ok {
		return nil, fmt.Errorf("json: expresion %q is not value", expr.String())
	}

	json := value.marshal()
	return newStringExpr(json, conf.line), nil
}

/******** unmarshal executor ********/

type unmarshalExecutor struct{}

func (unmarshalExecutor) execute(conf *recordExpr, tdef typedef) (expression, error) {
	strValue, ok :=  conf.readString("value")
	if !ok {
		return nil, fmt.Errorf("json: missing string value")
	}
	escaped := unQuote(strValue)
	return unmarshal(tdef, escaped)
}

/******** random executor ********/

type randomGeneratorExecutor struct {}

func (randomGeneratorExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	multiplier, ok1 := conf.readInt("multiplier")
	increment, ok2 := conf.readInt("increment")
	modulo, ok3 := conf.readInt("modulo")

	if !(ok1 && ok2 && ok3) {
		//Park–Miller generator (minimal standard, multiplicative)
		multiplier = 16807
		increment = 0
		modulo = 2147483647
	}

	seed, ok := conf.readInt("seed")
	if !ok {
		return nil, fmt.Errorf("random: missing seed")
	}
	return &randomGeneratorHandle {
		emptyHandle: emptyHandle{baseNode: baseNode{line: conf.line}},
		multiplier: multiplier,
		increment: 	increment,
		modulo: 	modulo,
		seed:		seed,
	}, nil
}

/******** memory reader executor ********/

type memoryReaderExecutor struct{}

func (memoryReaderExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	registryID, ok := conf.readString("registryID")
	if !ok {
		return eHandle, errors.New("memory: missing registryID definition")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	rec, err := findRecord(registryID)
	if err != nil {
		return nil, err
	}

	field, ok := conf.readString("field")
	if !ok {
		return rec, nil
	}

	expr, ok := rec.exprMap[field]
	if !ok {
		return nil, fmt.Errorf("unknown field: %q", field)
	}

	return expr, nil
}

/******** memory writer executor ********/

type memoryWriterExecutor struct{}

func (memoryWriterExecutor) execute(conf *recordExpr) (expression, error) {
	registryID, ok := conf.readString("registryID")
	if !ok {
		return nil, errors.New("memory: missing registryID definition")
	}

	field, ok := conf.readString("field")
	if !ok {
		field = ""
	 	//return nil, errors.New("memory: missing field")
	}

	h := &memoryHandle{
		emptyHandle: emptyHandle{baseNode: baseNode{line: conf.line}},
		registryID: registryID,
		field: field,
	}
	return h, nil
}

/******** mqtt subscribe executor ********/

type mqttSubscriberExecutor struct{}

func (mqttSubscriberExecutor) execute(conf *recordExpr, tdef typedef) (expression, error) {

	id, ok := conf.readString("id")
	if !ok {
		return nil, errors.New("mqtt: missing mqtt id")
	}

	broker, _ := conf.readString("address")
	client, _ := conf.readString("client")
	username, _ := conf.readString("username")
	password, _ := conf.readString("password")
	size, _ := conf.readInt("size")

	topic, _ := conf.readString("topic")

	qos, ok := conf.readInt("qos")
	if !ok {
		qos = 1
	}

	policyString, _ := conf.readString("policy")

	var policy sysio.OverflowPolicy
	switch policyString {
		case "Block":
			policy = sysio.Block
		case "DropNewest":
			policy = sysio.DropNewest
		case "DropOldest":
			policy = sysio.DropOldest
		case "Latest":
			policy = sysio.Latest
		default:
			policy = sysio.Latest
	}

	mqttClient, err := sysio.NewMQTTClient(
							id,
							broker,
							client,
							username,
							password,
							policy,
							size,
						)
	if err != nil {
		return nil, err
	}

	err = mqttClient.Subscribe(topic, byte(qos))

	if err != nil {
		return nil, err
	}

	msg, err := mqttClient.Receive(topic)
	if err != nil {
		return tdef.defaultValue(), err
	}

	switch st := tdef.(type) {
		case *stringType:
			return newStringExpr(string(msg.Payload), st.line), nil
		default:
			return tdef.defaultValue(), fmt.Errorf("expected string type; found %q instead.", tdef.String())
	}
}

/******** mqtt publish executor ********/

type mqttPublisherExecutor struct{}

func (mqttPublisherExecutor) execute(conf *recordExpr) (expression, error) {
	id, ok := conf.readString("id")
	if !ok {
		return nil, errors.New("mqtt: missing mqtt id")
	}

	broker, _ := conf.readString("address")
	client, _ := conf.readString("client")
	username, _ := conf.readString("username")
	password, _ := conf.readString("password")
	size, _ := conf.readInt("size")

	topic, _ := conf.readString("topic")

	qos, ok := conf.readInt("qos")
	if !ok {
		qos = 1
	}

	retained, _ := conf.readBool("retained")

	policyString, _ := conf.readString("policy")

	var policy sysio.OverflowPolicy
	switch policyString {
		case "Block":
			policy = sysio.Block
		case "DropNewest":
			policy = sysio.DropNewest
		case "DropOldest":
			policy = sysio.DropOldest
		case "Latest":
			policy = sysio.Latest
		default:
			policy = sysio.Latest
	}

	mqttClient, err := sysio.NewMQTTClient(
							id,
							broker,
							client,
							username,
							password,
							policy,
							size,
						)

	h := &mqttPublisherHandle {
		emptyHandle:	emptyHandle{baseNode: baseNode{line: conf.line}},
		mqttClient: 	mqttClient,
		topic:			topic,
		qos:			byte(qos),
		retained:		retained,
	}

	return h, err
}

/******** io/open executor ********/

type openFileExecutor struct{}

// parses { path: string, perms?: string } and opens a file handle.
func (openFileExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	path, ok := conf.readString("path")
	if !ok || path == "" {
		return nil, errors.New("io/open: missing path")
	}
	perms, _ := conf.readString("perms")
	if perms == "" {
		perms = "read"
	}

	flag, _ := flagsFor(perms)
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return nil, fmt.Errorf("io/open %q: %w", path, err)
	}

	h := &fileHandle{
		emptyHandle: emptyHandle{baseNode: baseNode{line: conf.line}},
		f:  sysio.NewFile(
				f,
				flag&os.O_RDONLY == os.O_RDONLY || flag&os.O_RDWR != 0,
				flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0,
			),
		path: path,
		perms: perms,
	}
	return h, nil
}

func flagsFor(perms string) (flag int, mode string) {
	switch perms {
	case "read":
		return os.O_RDONLY, "read"
	case "truncate":
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "write"
	case "append":
		return os.O_WRONLY | os.O_CREATE | os.O_APPEND, "write"
	case "create":
		return os.O_WRONLY | os.O_CREATE, "write"
	case "update":
		return os.O_RDWR | os.O_CREATE, "rw"
	default:
		return os.O_RDONLY, "read"
	}
}

/******** sleep executor ********/

type sleepExecutor struct {}
func (sleepExecutor) execute(rec *recordExpr) (expression, error) {
	unit, _ := rec.readString("unit")
	switch unit {
		case "ns", "us", "μs", "ms", "s", "m", "h":
		default:
			unit = "ms"
	}
 
	return &sleepHandle{ unit: unit }, nil
}

type timeExecutor struct {}
func (timeExecutor) execute(_ *recordExpr, _ typedef) (expression, error) {
	now := time.Now()
	tString := fmt.Sprintf("%s", now.String())
	return newStringExpr(tString, 0), nil
}

/******** compile executor ********/

type compileExecutor struct {}

func (compileExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	file, ok := conf.readString("program")
	if !ok {
		return nil, errors.New("runtime/compile: missing program")
	}

	p := NewProgram(file)
	log := p.Parse()

 	if log.HasErrors() {
        return nil, util.JoinSep("\n", errors.New("runtime/compile:"), log.Errors())
    }

    cfg := util.ParseFlags(nil, nil)

    elog, rlog := p.Validate(cfg)
    if elog.HasErrors() {
        return nil, util.JoinSep("\n", errors.New("runtime/compile:"), elog.Errors())
    }

    if rlog.HasReports() {
        rlog.PrintReports()
    }

	h := 	&programHandle {
				emptyHandle: emptyHandle{baseNode: baseNode{line: conf.line}},
				prog: p,
			}
	return h, nil
}

/**********************************************************************************
 * Handles
 **********************************************************************************/

type handle interface {
	expression
	output(expression) error
	input(typedef) (expression, error)
}

/**********************************************************************************
 * record handle
 **********************************************************************************/

func (re *recordExpr) output(v expression) error {
	srv, ok1 := re.readString("service")
	ac, ok2 := re.readString("action")

	var err1 error
	if !ok1 {
		err1 = fmt.Errorf("no service definition in io configuration")
	}

	var err2 error
	if !ok2 {
		err2 = fmt.Errorf("no action definition in io configuration")
	}

	if !ok1 || !ok2 {
		return util.JoinSep("; ", err1, err2)
	}

	exec, ok := findOutputExecutor(srv, ac)

	if !ok {
		return fmt.Errorf("undefined output io operation %s.%s", srv, ac)
	}
	
	h, err := exec.execute(re)
	if err != nil {
		return err
	}
	return h.(handle).output(v)
}

func (re *recordExpr) input(tdef typedef) (expression, error) {
	srv, ok1 := re.readString("service")
	ac, ok2 := re.readString("action")

	var err1 error
	if !ok1 {
		err1 = fmt.Errorf("no service definition in io configuration")
	}

	var err2 error
	if !ok2 {
		err2 = fmt.Errorf("no action definition in io configuration")
	}

	if !ok1 || !ok2 {
		return tdef.defaultValue(), util.JoinSep("; ", err1, err2)
	}

	exec, ok := findInputExecutor(srv, ac)

	if !ok {
		return tdef.defaultValue(), fmt.Errorf("undefined input io operation %s.%s", srv, ac)
	}
	
	h, err := exec.execute(re, tdef)
	if err != nil {
		return tdef.defaultValue(), err
	}
	td := h.getType()
	if !tdef.subtypeOf(td) {
		err := fmt.Errorf("expecting type %q; instead found type %q", tdef.String(), td.String())
		return tdef.defaultValue(), err
	}
	return h, err
}

/**********************************************************************************
 * empty handle
 **********************************************************************************/

type emptyHandle struct {
	baseNode
}

var eHandle *emptyHandle = &emptyHandle{}

func newEmptyHandle(line int) *emptyHandle {
	return &emptyHandle{
		baseNode: baseNode{line: line},
	}
}

func (h *emptyHandle) output(v expression) error { return nil }
func (h *emptyHandle) input(tdef typedef) (expression, error) { return tdef.defaultValue(), nil }
func (h *emptyHandle) close() error { return nil }
func (h *emptyHandle) getType() typedef { return newioType(h.line) }
func (h *emptyHandle) typeCheck(_ *typeCheckContext, _ util.ErrorLog) {}
func (h *emptyHandle) projectionCheck(_ *projectionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (h *emptyHandle) expressionCheck(_ *expressionCheckContext, _ util.ErrorLog) typedef { return h.getType() }
func (h *emptyHandle) sessionCheck(_ *sessionCheckContext, _ util.ErrorLog, _ util.ReportLog) {}
func (h *emptyHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *emptyHandle) operation(_ *evaluationContext, _ expression, _ string) expression { return nil }
func (h *emptyHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (_ *emptyHandle) String() string { return "io emptyHandle { }" }

/**********************************************************************************
 * file handle
 **********************************************************************************/

type fileHandle struct {
	emptyHandle
	f 		*sysio.File

	path 	string
	perms 	string
}

func (h *fileHandle) output(v expression) error {
	s := v.String()
	escaped := unQuote(s)
	_, err := h.f.Write([]byte(escaped))
	return err
}

func unQuote(s string) string {
	if (len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\''))) == false {
		s = "\"" + s + "\""
	}
	if escaped, err := strconv.Unquote(s); err == nil {
		return escaped
	}
	return s[1:len(s)-1]
}

func (h *fileHandle) input(tdef typedef) (expression, error) {
	line, err := h.f.ReadLine()
	if err != nil {
		return tdef.defaultValue(), err 
	}
	text := strings.TrimRight(line, "\r\n")

	switch st := tdef.(type) {
		case *stringType:
			return newStringExpr(text, st.line), nil
		default:
			err := fmt.Errorf("expecting string type; found %q instead.", tdef.String())
			return tdef.defaultValue(), err
	}
}

func (h *fileHandle) close() error {
	return h.f.Close()
}

func (h *fileHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *fileHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *fileHandle) String() string {
	return "io fileHandle { path: " + h.path + ", perms: " + h.perms + " }"
}

/******** stdio handle ********/

var stdoutHandle *fileHandle = &fileHandle {
	emptyHandle: emptyHandle{baseNode: baseNode{line: 0}},
	f: sysio.NewFile(os.Stdout, false, true),
	path: "Stdout",
	perms: "append",
}

var stdinHandle *fileHandle = &fileHandle {
	emptyHandle: emptyHandle{baseNode: baseNode{line: 0}},
	f: sysio.NewFile(os.Stdin, true, false),
	path: "Stdin",
	perms: "read",
}

/**********************************************************************************
 * memory handle
 **********************************************************************************/

type memoryHandle struct {
	emptyHandle
	registryID string
	field string
}

func (h *memoryHandle) output(value expression) error {
	if h.field == "" {
		rec, ok := value.(*recordExpr)
		if !ok || rec == nil {
			return errors.New("memory: expecting record type")
		}
		_, err := registerSchema(h.registryID, rec)
		return err
	}
	return writeRegistry(h.registryID, h.field, value)
}

func (h *memoryHandle) input(tdef typedef) (expression, error) {
	expr, err := readRegistry(h.registryID, h.field)
	if err != nil {
		return tdef.defaultValue(), err
	}

	td := expr.getType()
	if td == nil {
		err := fmt.Errorf("unexpected expression on field: %q", h.field)
		return tdef.defaultValue(), err
	}

	if !tdef.subtypeOf(td) {
	 	err := fmt.Errorf("Expecting subtype of %q; instead found type %q", tdef.String(), td.String())
		return tdef.defaultValue(), err
	}
	return expr, err
}

func (h *memoryHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *memoryHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *memoryHandle) String() string {
	return "io memoryHandle { registryID: " + h.registryID + " }"
}

/**********************************************************************************
 * mqtt handle
 **********************************************************************************/

type mqttPublisherHandle struct {
	emptyHandle
	mqttClient 		*sysio.MQTTClient
	topic			string
	qos				byte
	retained		bool
}

func (h *mqttPublisherHandle) output(value expression) error {
	escaped := unQuote(value.String())
	b := []byte(escaped)
	return h.mqttClient.Publish(h.topic, h.qos, h.retained, b)
}

func (h *mqttPublisherHandle) input(tdef typedef) (expression, error) {
	return tdef.defaultValue(), fmt.Errorf("invalid input operation for mqtt publish")
}

func (h *mqttPublisherHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *mqttPublisherHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *mqttPublisherHandle) String() string {
	return fmt.Sprintf("io mqttPublisherHandle { publish: %v, qos: %v, retained: %v }", h.topic, h.qos, h.retained)
}

/**********************************************************************************
 * sleep handle
 **********************************************************************************/

type sleepHandle struct {
	emptyHandle
	unit string
}

func (h *sleepHandle) output(value expression) error {
	var val float64
	switch v := value.(type) {
		case *intExpr:
			val = float64(v.integer)
		case *floatExpr:
			val = v.floating
		default:
			return fmt.Errorf("invalid input for sleep: %q; expecting floating or integer", value.String())
	}

	if val < 0 {
        return fmt.Errorf("sleep duration must be non-negative")
    }

	var scale time.Duration
	switch h.unit {
		case "ns":
			scale = time.Nanosecond
		case "us", "μs":
			scale = time.Microsecond
		case "ms":
			scale = time.Millisecond
		case "s":
			scale = time.Second
		case "m":
			scale = time.Minute
		case "h":
			scale = time.Hour
		default:
			scale = time.Millisecond
	}
	duration := time.Duration(math.Round(val * float64(scale)))
	time.Sleep(duration)
	return nil
}

func (_ *sleepHandle) input(tdef typedef) (expression, error) {
	return tdef.defaultValue(), fmt.Errorf("invalid input operation for sleep")
}

func (h *sleepHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *sleepHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *sleepHandle) String() string {
	return "io sleepHandle { unit: " + h.unit + " }"
}

var sHandle *sleepHandle = &sleepHandle{}

/**********************************************************************************
 * random Generator handle
 **********************************************************************************/

type randomGeneratorHandle struct {
	emptyHandle
	multiplier 	int
	increment	int
	modulo 		int
	seed 		int
}

func (_ *randomGeneratorHandle) output(_ expression) error {
	return fmt.Errorf("cannot use random generator as output")
}

func (h *randomGeneratorHandle) input(_ typedef) (expression, error) {
	random := ((h.seed * h.multiplier) + h.increment) % h.modulo
	srandom := fmt.Sprintf("%d", random)
	expr := newIntExpr(srandom, h.line)
	h.seed = random
	return expr, nil
}

func (h *randomGeneratorHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *randomGeneratorHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *randomGeneratorHandle) String() string {
	return fmt.Sprintf("io randomGeneratorHandle { multiplier: %d; increment: %d; modulo: %d; seed: %d}", h.multiplier, h.increment, h.modulo, h.seed)
}

/**********************************************************************************
 * program handle
 **********************************************************************************/

type programHandle struct {
	emptyHandle
	prog *program
}

func (h *programHandle) output(_ expression) error {
	h.prog.Execute()
	return nil
}

func (_ *programHandle) input(tdef typedef) (expression, error) {
	return tdef.defaultValue(), fmt.Errorf("invalid input operation for program")
}

func (h *programHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *programHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *programHandle) String() string {
	return "io program { program: " + h.String() + " }"
}

/**********************************************************************************
 * api handle
 **********************************************************************************/

type apiHandle struct {
	emptyHandle
	api api.API
	request string
	response string
}

func (_ *apiHandle) output(_ expression) error {
	return fmt.Errorf("cannot use api as output")
}

func (h *apiHandle) input(tdef typedef) (expression, error) {
	request := unQuote(h.request)
	h.response = h.api.Request(request)
	escaped := unQuote(h.response)
	return unmarshal(tdef, escaped)
	//return newStringExpr(h.response, h.line), nil
}

func (h *apiHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *apiHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *apiHandle) String() string {
	return fmt.Sprintf("io apiHandle { api: %v; request: %v; response: %v }", h.api.String(), h.request, h.response)
}

/******************************************************************************
 *  Implement actions for record expression as io configuration
 ******************************************************************************/

 func (r *recordExpr) readString(label string) (string, bool) {
    expr, ok := r.exprMap[label]
    if ok {
        sExpr, ok := expr.(*stringExpr)
        if ok {
            return sExpr.stringVal, true
        }
    }
    return "", false
}

func (r *recordExpr) readInt(label string) (int, bool) {
    expr, ok := r.exprMap[label]
    if ok {
        iExpr, ok := expr.(*intExpr)
        if ok {
            return iExpr.integer, true
        }
    }
    return 0, false
}

func (r *recordExpr) readBool(label string) (bool, bool) {
    expr, ok := r.exprMap[label]
    if ok {
    	switch expr.(type) {
    		case *trueExpr:
    			return true, true
    		case *falseExpr:
    			return false, true
    		default:
    			return false, false
    	}
    }
    return false, false
}

/******************************************************************************
 *  Registry for records 
 ******************************************************************************/

type recordEntry struct {
	rec *recordExpr
	schema typedef
}

type recordRegistry struct {
	entries map[string]*recordEntry
	mu sync.Mutex
}

var (
	registry = recordRegistry{ entries: make(map[string]*recordEntry) }
)

func findRecord(registryID string) (*recordExpr, error) {
	entry, ok := registry.entries[registryID]
	if !ok {
		return nil, fmt.Errorf("unregistered schema: %q", registryID)
	}
	return entry.rec, nil
}

func registerSchema(registryID string, value *recordExpr) (*recordExpr, error) {
	schema := value.getType()
	if schema == nil {
		return nil, fmt.Errorf("unrecognised schema for registry id: %q", registryID)
	}
	if entry, ok := registry.entries[registryID]; ok {
		if !schema.subtypeOf(entry.schema) {
			return nil, fmt.Errorf("unmatched schema for registry id: %q", registryID)
		}
		entry.rec = value
	} else {
		registry.entries[registryID] = &recordEntry{rec: value, schema: schema}
	} 
	return value, nil
}

func writeRegistry(registryID string, field string, value expression) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	rec, err := findRecord(registryID)
	if err != nil {
		return err
	}

	expr, ok := rec.exprMap[field]
	if !ok {
		return fmt.Errorf("unknown field: %q.", field)
	}

	tdef1 := value.getType()
	tdef2 := expr.getType()
	if !(tdef1.subtypeOf(tdef2) && tdef2.subtypeOf(tdef1)) {
		return fmt.Errorf("types for %q and %q do not match", expr.String(), value.String())
	}

	//let there be write
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for i := range rec.labels {
		if field == rec.labels[i] {
			rec.expressions[i] = value
			rec.exprMap[field] = value
		}
	}
	return nil
}

func readRegistry(registryID string, field string) (expression, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	rec, err := findRecord(registryID)
	if err != nil {
		return nil, err
	}

	expr, ok := rec.exprMap[field]
	if !ok {
		return nil, errors.New("Unknown field: " + field)
	}

	return expr, nil
}
