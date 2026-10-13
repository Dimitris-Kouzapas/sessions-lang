grammar sessions;

module          :   imports declaration* EOF
                ;

imports         :   (importModule SEMICOLON?)*
                ;

importModule    :   IMPORT path (AS ID)?
                ;

path            :   ID (DOT ID)*
                ;

declaration     :   VAL name AS abstraction
                |   VAL name AS primaryTerm
                |   TYPE name abstractionType
                |   TYPE name configuration
                |   TYPE name recordType
                |   TYPE name channelType
                |   name COLON abstractionTypeDef
                |   declarationSugar
                ;

declarationSugar:   PROC name (LBRA participant RBRA)? participant* DOT process
                |   PROC name receiveMacro participant* DOT process
                |   RECORD name primaryRecord
                |   SHARED name channel
                ;

name            :   ID
                ;

participant     :   ID
                ;

variableDef     :   variable type
                ;

variable        :   ID
                ;

application     :   term (LBRA term RBRA)? participant*
                |   term sendMacro participant*
                ;

abstraction     :   PROC (LBRA participant RBRA)? participant* DOT process
                |   PROC receiveMacro participant* DOT process
                ;

concurrent      :	CONC WITH (ROLE name AS CURRENT SEMICOLON?)? (ROLE participant AS application SEMICOLON?)+
				|	CONC WITH LBRA (ROLE name AS CURRENT SEMICOLON?)? (ROLE participant AS application SEMICOLON?)+ RBRA
                ;

acceptProc      :   ACC LPAR term RPAR WITH ROLE participant AS application
				|	ACC LPAR term RPAR WITH LBRA ROLE participant AS application RBRA
                ;

request         :	REQ LPAR term RPAR WITH ROLE participant AS CURRENT
                |   REQ LPAR term RPAR WITH LBRA ROLE participant AS CURRENT RBRA
                |   REQ LPAR term RPAR WITH ROLE participant AS application
				|	REQ LPAR term RPAR WITH LBRA ROLE participant AS application RBRA
                ;

recursion       :   REC (LPAR term RPAR WITH ROLE participant AS)? application
                ;

process         :   blockProc
                |   sequenceProc
                |   ifThenElseProc
                |   selectProc
                |   branchProc
                |   recursion
                |   terminateProc
                ;

blockProc       :   LBRA process RBRA
                ;

sequenceProc    :   prefix SEMICOLON? process?
                |   concurrent SEMICOLON? process?
                |   request SEMICOLON? process?
                |   acceptProc SEMICOLON? process?
                |   call SEMICOLON? process?
                ;

prefix          :   receive
                |   out
                |   inp
                |   let
                |   closeChannel
                ;

receive         :   variableDef ARROW participant
                |   variableDef ARROW send
                |   send
                ;

send            :   participant (ARROW logicalOr)+
                |   LPAR receive RPAR (ARROW logicalOr)*
                ;

out             :   OUT (LSQ term RSQ)? (ARROW logicalOr)+
                ;

inp             :   variableDef ARROW INP (LSQ term RSQ)?
                ;

let             :   LET variable type? AS logicalOr
                ;

closeChannel    :   term ARROW CLOSE
                ;

call            :   variableDef (COMMA variableDef)* EQUALS term LPAR (logicalOr (COMMA logicalOr)*)? RPAR
                ;

ifThenElseProc  :   IF logicalOr THEN process ELSE process
                ;

selectProc      :   SELECT participant OF (logicalOr ARROW labelExpression COLON process)* labelExpression COLON process
				|	SELECT participant OF LBRA (logicalOr ARROW labelExpression COLON process)* labelExpression COLON process RBRA
                |   participant ARROW labelExpression process?
                ;

branchProc      :   BRANCH participant OF (labelExpression COLON process)* labelExpression COLON process
				|	BRANCH participant OF LBRA (labelExpression COLON process)* labelExpression COLON process RBRA
                ;

labelExpression :   LABEL
                ;

terminateProc   :   TERMINATE SEMICOLON?
                ;

logicalOr       :   logicalOr LOR logicalAnd
                |   logicalAnd
                ;

logicalAnd      :   logicalAnd LAND equalityExpr
                |   equalityExpr
                ;

equalityExpr    :   equalityExpr eqop relationalExpr
                |   relationalExpr
                ;

eqop            :   EQ
                |   NEQ
                ;

relationalExpr  :   relationalExpr relop sumExpr
                |   sumExpr
                ;

relop           :   LT
                |   GT
                |   LTE
                |   GTE
                ;

sumExpr         :   sumExpr sumop multExpr
                |   multExpr
                ;

sumop           :   PLUS
                |   MINUS
                ;

multExpr        :   multExpr mulop unary
                |   unary
                ;

mulop           :   MUL
                |   DIV
                |   MOD
                |   POW
                ;

unary           :   NOT unary
                |   sumop unary
                |   term
                ;

term            :   literal
                |   variable                                      // variable
                |   listExpr                                      // list
                |   term listSlice
                |   term LSQ logicalOr RSQ                        // list access
                |   conditionalExpr
                |	abstraction
                |   record
                |   rename
                |   channel
                |   term DOT ID
                |   macro
                |   LPAR logicalOr RPAR
                ;

literal         :   INT_LIT
                |   FLOAT_LIT
                |   TRUE | FALSE
                |   STRING_LIT
                ;

listExpr        :   LSQ (logicalOr (COMMA logicalOr)*)? COMMA? RSQ
                ;

listSlice       :   LSQ left=logicalOr? COLON right=logicalOr? RSQ
                ;

conditionalExpr :   IF logicalOr THEN logicalOr ELSE logicalOr
                ;

record          :   LBRA (ID COLON logicalOr (SEMICOLON | COMMA)?)* RBRA
                ;

channel         :   SYNC LBRA CHANNEL COLON configurationDef (SEMICOLON | COMMA)? REQUESTER COLON participant (SEMICOLON | COMMA)? RBRA
                |   ASYNC participant* LBRA RBRA
                ;

rename          :   LSQ participant* RSQ term
                ;


macro           :   SEND LSQ prefixMacro (COMMA prefixMacro)* RSQ
                ;

prefixMacro     :   logicalOr
                |   variableDef
                ;

sendMacro       :   LT logicalOr (COMMA logicalOr)* GT
                ;

receiveMacro    :   LPAR variableDef (COMMA variableDef)* RPAR
                ;

primaryTerm     :   sumop? literal
                |   primaryList
                |   primaryRecord
                |   channel
                |   macro
                ;

primaryList     :   LSQ (primaryTerm (COMMA primaryTerm)*)? COMMA? RSQ
                ;

primaryRecord   :   LBRA (ID COLON primaryTerm (SEMICOLON | COMMA)?)* RBRA
                ;

////////////////////////////////////////////////////////////////////////////////

type            :   primitiveType
                |   LSQ type RSQ
                |   recordType
                |   channelType
                |   abstractionType
                |   nameType
                |   ioType
                |   LPAR type RPAR
                ;

primitiveType   :   BOOLEAN
                |   INT
                |   FLOAT
                |   STRING
                ;

nameType        :   name
                |   name DOT name
                ;

ioType          :   IO
                ;

abstractionType :   (INIT ID DOT local PIPE)? session
                ;

session         :   localAbstraction
                |   projection
                ;

configuration   :   globalDef
                |   localContext
                ;

recordType      :   RECORD LBRA (ID type (SEMICOLON | COMMA)?)* RBRA
                ;

channelType     :   SHARED LBRA configurationDef CHANNEL (SEMICOLON | COMMA)? ID REQUESTER (SEMICOLON | COMMA)? RBRA
                |   SHARED LBRA localSet CHANNEL (SEMICOLON | COMMA)? (ID REQUESTER (SEMICOLON | COMMA)?)? RBRA
                |   SHARED ID* LBRA RBRA
                ;

abstractionTypeDef
                :   abstractionType
                |   nameType
                ;

configurationDef:   configuration
                |   nameType
                ;

localSet        :   LBRA (ID COLON local COMMA?)+ RBRA
                ;

/*********************
 *   local types
 *********************/


projection      :   configurationDef LPAR ID RPAR
                ;

localAbstraction:   LOCAL ID* DOT local
                ;

labelType       :   LABEL
                ;

local           :   sendLocal
                |   receiveLocal
                |   selectLocal
                |   branchLocal
                |   END
                |   REC
                ;

sendLocal       :   ID SEND type (DOT local)?
                ;

receiveLocal    :   ID RECEIVE type (DOT local)?
                ;

selectLocal     :   SELECT ID LBRA labelLocal RBRA (OR LBRA labelLocal RBRA)*
                ;

branchLocal     :   BRANCH ID LBRA labelLocal RBRA (OR LBRA labelLocal RBRA)*
                ;

labelLocal      :   labelType COLON local
                ;

/*********************
 *   local context
 *********************/

localContext    :   CONTEXT localSet
                ;

/*********************
 *   global types
 *********************/

globalDef       :   GLOBAL ID* DOT global
                ;

global          :   pass
                |   choice
                |   END
                |   REC
                ;

pass            :   ID PASS ID COLON type (DOT global)?
                ;

choice          :   ID PASS ID LBRA labelGlobal RBRA (OR LBRA labelGlobal RBRA)*
                ;

labelGlobal     :   labelType COLON global
                ;

/******************************************************************************
 *  Lexer
 ******************************************************************************/

IMPORT:         'import';
PROC:           'proc';
VAL:            'val';
TYPE:           'type';
LET:			'let';
AS:				'as';
CONC:           'conc';
REQ:            'req';
ACC:            'acc';
REC:            'rec';
CURRENT:		'current';
ROLE:			'role';
WITH:			'with';
SELECT:         'select';
BRANCH:         'branch';
OF:				'of';
IF:             'if';
THEN:           'then';
ELSE:           'else';
TERMINATE:      'term';

IO:             'io';

SHARED:         'shared';
CHANNEL:        'chan';
REQUESTER:      'requester';
SYNC:			'sync';
ASYNC:			'async';

COLON:          ':';
SEMICOLON:      ';';
DOT:            '.';
COMMA:          ',';
ARROW:          '<-';

LPAR:           '(';
RPAR:           ')';
LSQ:            '[';
RSQ:            ']';
LBRA:           '{';
RBRA:           '}';

LOR:            '||';
LAND:           '&&';
NOT:            'not';

// eqop
EQ:             '==';
NEQ:            '!=';

// relop
LT:             '<';
GT:             '>';
LTE:            '<=';
GTE:            '>=';

// sumop
PLUS:           '+';
MINUS:          '-';

// mulop
MUL:            '*';
DIV:            '/';
MOD:            '%';
POW:            '^';

EQUALS:         '=';

OUT:            'out';
INP:            'inp';
CLOSE:          '.close';

fragment DIGITS:
                [0-9]+ ;
fragment EXP:   [eE] [+\-]? DIGITS ;

FLOAT_LIT:      DIGITS '.' DIGITS (EXP)?
         |      DIGITS '.' (EXP)?
         |      '.' DIGITS (EXP)?
         |      DIGITS EXP
         ;

INT_LIT:        DIGITS ;

TRUE:           'true';
FALSE:          'false';
//STRING_LIT:     '"'~('"')*'"';
STRING_LIT : '"' ( '\\' . | ~["\\] )* '"' ;

// Types

INT:            'int';
FLOAT:          'float';
STRING:         'string';
BOOLEAN:        'bool';

RECORD:         'record';

LOCAL:          'local';
INIT:           'init';
PIPE:           '|';
SEND:           '!';
RECEIVE:        '?';
OR:             'or';
END:            'end';

GLOBAL:         'global';
PASS:           '->';
CONTEXT:        'context';

// Identifiers

ID:             [a-z][a-zA-Z0-9_]*;
LABEL:          [A-Z][a-zA-Z0-9_]*;

COMMENTS:       '//' ~[\r\n]* -> skip;
BLOCK_COMMENT:  '/*' ( . | '\r' | '\n' )*? '*/' -> skip ;
WS:             [ \t\n\r] -> skip;
