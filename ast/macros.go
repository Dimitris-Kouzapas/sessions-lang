package ast
import "fmt"

func newCall(exprs []expression, abstrTerm expression, variables []*variableExpr, proc process, line int) *concurrentProc {
    name  := fmt.Sprintf("*(%v@line:%v)", abstrTerm.String(), line)
    pexpr := newParticipantExpr(name, line)

    var lexpr expression = pexpr
    for _, rexpr := range exprs {
        lexpr = newSendExpr(lexpr, rexpr, line)
    }

    var expr seqExpr
    for _, variable := range variables {
        expr = newReceiveExpr(variable, lexpr, line)
        lexpr = expr
    }

    proc = newSequentialProc(expr, proc, line)
    participant := newParticipantExpr("user", line)

    appl := newApplication(abstrTerm, []*participantExpr{ participant }, nil, line)
    return newConcurrentProc(participant, []*participantExpr{pexpr}, []*application{appl}, proc, line)
}

func newSeqProcMacro(exprs []expression, body process, line int) ([]*participantExpr, process) {
    name := fmt.Sprintf("*(u@line:%v)", line)
    pexpr := newParticipantExpr(name, line)
    initParameters := []*participantExpr{ pexpr }

    if len(exprs) != 0 {
        var lexpr expression = pexpr
        var sexpr seqExpr
        for _, rexpr := range exprs {
            switch re := rexpr.(type) {
                case *variableExpr:
                    if re.tdef != nil {
                        sexpr = newReceiveExpr(re, lexpr, line)
                    } else {
                        sexpr = newSendExpr(lexpr, rexpr, line)
                    }
                default:
                    sexpr = newSendExpr(lexpr, rexpr, line)
            }
            lexpr = sexpr
        }
        body = newSequentialProc(sexpr, body, line)
    }
    
    return initParameters, body
}
