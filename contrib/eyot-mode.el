; useful refs
;   https://www.emacswiki.org/emacs/GenericMode
;   https://github.com/typester/emacs/blob/master/lisp/generic-x.el
										;
; TODO
; - chars need colouring
(define-generic-mode 
    ; mode name
    'eyot-mode

    ; comment start
    '("//" ("/*" . "*/" ))

    ; keywords
    '(
        "fn" "partial" "_"
        "struct" "self" "true" "false"  "return" "if" "else" "elseif" "and" "not" "or" "placeholder" "new" "range"
        "for" "while" "break"
        "let" "const"
        "bool" "string" "i64" "f32" "f64" "char"
		"null"
        "print" "print_ln"
        "cpu" "gpu" "send" "receive" "worker" "drain"
        "pixel" "vertex" "pipeline" "gpubuiltin"
        "import" "as"
     )

    ; misc font locks
    '(
        ("[0-9]+" . 'font-lock-variable-name-face)
        ("^fn\\s-+\\([A-Za-z0-9_]+\\)" (1 font-lock-function-name-face))
        ("^\\s-*(cpu|gpu)\\s-+fn\\s-+\\([A-Za-z0-9_]+\\)" (2 font-lock-function-name-face))
        ("^\\s-*struct\\s-+\\([A-Za-z0-9_]+\\)" (1 font-lock-type-face))
    )

    ; file types
    '("\\.ey$")

    ; functions to call
    nil

    ; doc string
    "A mode for foo files"
)
