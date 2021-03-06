package moonshine

import (
	"regexp"
	"strings"
	"strconv"
)

var RECLAIMSTRING = regexp.MustCompile("^([\\s\\S]*?)(!STR!([0-9]+)!STR!)([\\s\\S]*)$") //Has s/e anchors
var ESCAPEDOLLARS = regexp.MustCompile("\\$")
var REPLACE020 = regexp.MustCompile("\020")

//Moonshine
var COMMENT = regexp.MustCompile("--((?:[^\\n])*)\\n")
var BLOCKCOMMENT = regexp.MustCompile("---((?:.|\\n)*?)---")
var ISCONDITION = regexp.MustCompile("\\sis\\s")
var ISNTCONDITION = regexp.MustCompile("\\sisnt\\s")
var FUNCACCESSOR = regexp.MustCompile("::")
var ZEROOP = regexp.MustCompile("((?:[a-zA-Z_]+(?:[a-zA-Z0-9_]*))|(?:\\([^)(\\n]+\\)))\\?")
var EXISACCESSOR = regexp.MustCompile("([a-zA-Z_]+(?:[a-zA-Z0-9_.?]*))\\?\\.")
var INCACCESSOR = regexp.MustCompile("([^\\s\\n]*)\\[(.*)##(.*)\\]")
var PINCACCESSOR = regexp.MustCompile("([^\\s\\n]*)\\[\\+\\]")
var WHITESPACEREMOVE = regexp.MustCompile("(?m)(?:\\n|\\r)+(?: |\\t)*(::|\\\\)")
var INCREMENTOPERATOR = regexp.MustCompile("\\+\\+")
var DOUBLESTATEMENTOP = regexp.MustCompile("([\t ]*)(.*)\\s&&\\s")

var KEY = regexp.MustCompile("(?:><)|(?:<>)")
var SETKEY = regexp.MustCompile(">>\\s?([\\s\\S]*)")
var RELEASEKEY = regexp.MustCompile("<<")

//translates moonshine to moonscript (where the magic happens)
func Translate(input string) (string, error) {
	local := input + "\n" //some breathing room

	//Delete comments
	local = BLOCKCOMMENT.ReplaceAllString(local, "")
	local = COMMENT.ReplaceAllString(local, "\n")

	//Hide Strings
	local, sArr := hideStrings(local)

	//Remove breaking whitespace
	local = WHITESPACEREMOVE.ReplaceAllString(local, "$1\020$2\020")
	local = REPLACE020.ReplaceAllString(local, "")

	//Change && to \n
	local = DOUBLESTATEMENTOP.ReplaceAllString(local, "$1\020$2\020\n$1\020")
	local = REPLACE020.ReplaceAllString(local, "")

	//Change " is " to " == "
	local = ISCONDITION.ReplaceAllString(local, " == ")

	//Change " isnt " to " != "
	local = ISNTCONDITION.ReplaceAllString(local, " != ")

	//Change "::" to "\"
	local = FUNCACCESSOR.ReplaceAllString(local, "\\")

	//Change a[.##.] to a[.#a.]
	local = INCACCESSOR.ReplaceAllString(local, "$1\020[$2\020#$1\020$3\020]")
	local = REPLACE020.ReplaceAllString(local, "")

	//Change a[+] to a[#a+1]
	local = PINCACCESSOR.ReplaceAllString(local, "$1\020[#$1\020+1]")
	local = REPLACE020.ReplaceAllString(local, "")

	//Change ++ to += 1
	local = INCREMENTOPERATOR.ReplaceAllString(local, "+=1")

	//Existential accessor, must come before zero op
	for {
		if EXISACCESSOR.MatchString(local) == false {break}
		local = EXISACCESSOR.ReplaceAllString(local, "($1 or {}).")
	}

	//Zero operator
	local = ZEROOP.ReplaceAllString(local, "($1 != \"\" and $1 != 0)")

	//Show Strings
	local, err := showStrings(local, sArr)
	if err != nil {return "", err}

	//use keys
	local, err = keys(local)
	if err != nil {return "", err}

	return local, nil
}

//writes all strings to array and replaces them with a token
func hideStrings(input string) (string, []string) {
	sArr := make([]string, 0, 0)

	isString := false
	stringOpener := ""
	hasEscape := false
	hasInterp := false
	lastCh := ""
	strBuff := ""
	locBuff := ""
	for _, c := range input {
		char, _ := strconv.Unquote(strconv.QuoteRuneToASCII(c))
    if isString {
			//Switch when string opened
			switch (char) {
				case "{":
					if lastCh == "#" {
						hasInterp = true
					}
					strBuff += char
				case "}":
					hasInterp = false
					strBuff += char
				case "\\": //escape symbol, toggle escaping
					hasEscape = !hasEscape
					strBuff += char
				case "'", "\"": //close string
					if (!hasEscape) && (!hasInterp) && stringOpener == char{
						isString = false
						if hasEscape {hasEscape = false}
						locBuff += char + "!STR!" + strconv.Itoa(len(sArr)) + "!STR!" + char
						sArr = append(sArr, strBuff)
						strBuff = "" //reset string buffer
					} else {
						strBuff += char
					}

					if (hasEscape) {
						if hasEscape {hasEscape = false}
					}

				default:
					strBuff += char
					if hasEscape {hasEscape = false}
			}

		} else {
			//Switch when string closed
			switch (char) {
				case "'", "\"": //open string
					if !hasEscape {
						isString = true
						stringOpener = char
					}
				default: //add char to nonstring buffer
					locBuff += char
			}

		}

		//record last character (for interpolation check)
		lastCh = char
	}

	return locBuff, sArr
}

//replaces all string tokens with their original value from an array
func showStrings(input string, sArr []string) (string, error) {
	local := input
	for {
		if RECLAIMSTRING.MatchString(local) == false {break}
		id, err := strconv.Atoi(RECLAIMSTRING.ReplaceAllString(local, "$3"))
		if err != nil {return "", err}
		get := "$1\020" + ESCAPEDOLLARS.ReplaceAllString(sArr[id],"$\020") + "$4"
		local = RECLAIMSTRING.ReplaceAllString(local, get) //the $1 doesnt like to be next to a string, so we put the space char code in
		local = REPLACE020.ReplaceAllString(local, "")
	}
	return local, nil
}

func keys(input string) (string, error) {

	local := ""
	buffer := ""
	lines := make([]string, 0, 0)
	for _, c := range input {
		char, err := strconv.Unquote(strconv.QuoteRuneToASCII(c))
		if err != nil {return "", err}
		if char == "\n" || char == "\r" {
			lines = append(lines, buffer)
			buffer = ""
		}
		buffer += char
	}

	key := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		//replace keys
		if KEY.MatchString(line) {
			if key != "" {
				lines[i] = KEY.ReplaceAllString(line, key)
			}
		}
		//store key
		if SETKEY.MatchString(line) {
			key = SETKEY.ReplaceAllString(line, "$1")
			key = key[1:] //remove newline at start
			lines[i] = ""
		}
		//unset key
		if RELEASEKEY.MatchString(line) {
			key = ""
			lines[i] = ""
		}
	}

	local = strings.Join(lines[:],"")
	return local, nil
}
