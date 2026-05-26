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

var inputExecutors = map[actionKey]inputHandleExecutor {
	actionKey{"io", "open"}: openFileExecutor{},
	actionKey{"io", "mqtt"}: mqttClientExecutor{},
	actionKey{"memory", "read"}: memoryReaderExecutor{},
	actionKey{"json", "marshal"}: marshalExecutor{},
	actionKey{"json", "unmarshal"}: unmarshalExecutor{},
	actionKey{"time", "current"}: timeExecutor{},
	actionKey{"random", "generator"}: randomGeneratorExecutor{},
}

var outputExecutors = map[actionKey]outputHandleExecutor{
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

/******** marshal executor ********/

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

/******** memory register executor ********/

// type memoryRegisterExecutor struct{}

// func (memoryRegisterExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
// 	value, ok := conf.readRecord("value")
// 	if !ok || value == nil {
// 		return nil, errors.New("memory: missing value")
// 	}

// 	registryID, ok := conf.readString("registryID")
// 	if !ok {
// 		return nil, errors.New("memory: missing registryID") 
// 	}

// 	_, err := registerSchema(registryID, value)
// 	return newTrueExpr(conf.line), err
// }

/******** memory reader executor ********/

type memoryReaderExecutor struct{}

func (memoryReaderExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	registryID, ok := conf.readString("registryID")
	if !ok {
		return eHandle, errors.New("memory: missing registryID definition")
	}

	//if !ok {
	//	return eHandle, errors.New("memory: missing field") 
	//}
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

/******** mqtt executor ********/

type mqttClientExecutor struct{}

func (mqttClientExecutor) execute(conf *recordExpr, _ typedef) (expression, error) {
	var brokers []string
	if broker, ok := conf.readString("address"); ok {
		brokers = append(brokers, broker)
	}
	client, _ := conf.readString("client")
	username, _ := conf.readString("username")
	password, _ := conf.readString("password")
	size, _ := conf.readInt("size")

	mqttClient, err1 := sysio.NewMQTTClient(brokers, client, username, password, size)

	topic, _ := conf.readString("topic")
	qos, ok := conf.readInt("qos")
	if !ok {
		qos = 1
	}

	var err2 error
	if mqttClient != nil {
		err2 = mqttClient.Subscribe(topic, byte(qos))
	}

	h := &mqttHandle {
		emptyHandle:	emptyHandle{baseNode: baseNode{line: conf.line}},
		mqttClient: 	mqttClient,
		topic:			topic,
		qos:			byte(qos),
	}

	return h, util.JoinSep(";", err1, err2)
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

	if !(ok1 && ok2) {
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

	return unmarshal(tdef, text)
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
 * sleep handle
 **********************************************************************************/

type mqttHandle struct {
	emptyHandle
	mqttClient *sysio.MQTTClient
	topic string
	qos byte
}

func (h *mqttHandle) output(value expression) error {
	escaped := unQuote(value.String())
	b := []byte(escaped)
	return h.mqttClient.Publish(h.topic, h.qos, true, b)
}

func (h *mqttHandle) input(tdef typedef) (expression, error) {
	if h.mqttClient == nil {
		return tdef.defaultValue(), fmt.Errorf("mqtt client not connected")
	}
	msg, ok := h.mqttClient.Receive(h.topic)
	if !ok {
		return tdef.defaultValue(), fmt.Errorf("did not receive msg from topic: %q", h.topic)
	}
	return unmarshal(tdef, string(msg.Payload))
}

func (h *mqttHandle) evaluate(_ *evaluationContext) expression { return h }
func (h *mqttHandle) prettyPrint(iw util.IndentedWriter) { iw.Print(h.String()) }
func (h *mqttHandle) String() string {
	return "io mqttHandle { topic: " + h.topic + ", " + "qos: " + string(h.qos) + " }"
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

/******************************************************************************
 *  Implement actions for record expression as io configuration
 ******************************************************************************/

// func (r *recordExpr) readRecord(label string) (*recordExpr, bool) {
//     expr, ok := r.exprMap[label]
//     if ok {
//         rExpr, ok := expr.(*recordExpr)
//         if ok {
//             return rExpr, true
//         }
//     }
//     return nil, false
// }

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
