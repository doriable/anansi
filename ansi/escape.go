package ansi

import "fmt"

// Escape identifies an ANSI control code, escape sequence, or control sequence
// as a Unicode codepoint:
// - U+0000-U+001F: C0 controls
// - U+0080-U+009F: C1 controls
// - U+EF20-U+EF2F: character set selection functions
// - U+EF30-U+EF3F: private ESCape-sequence functions
// - U+EF40-U+EF5F: non-standard ESCape-sequence functions
// - U+EF60-U+EF7E: standard ESCape-sequence functions
// -        U+EF7F: malformed ESC sequence
// - U+EFC0-U+EFFE: CSI functions
// -        U+EFFF: malformed CSI sequence
type Escape rune

// ESC returns an ESCape sequence identifier named by the given byte.
func ESC(b byte) Escape { return Escape(0xEF00 | 0x7F&rune(b)) }

// CSI returns a CSI control sequence identifier named by the given byte.
func CSI(b byte) Escape { return Escape(0xEF80 | rune(b)) }

// ESC returns the byte name of the ESCape sequence identified by this escape
// value, if any; returns 0 false otherwise.
func (id Escape) ESC() (byte, bool) {
	if 0xEF00 < id && id < 0xEF7F {
		return byte(id & 0x7F), true
	}
	return 0, false
}

// CSI returns the byte name of the CSI control sequence identified by this
// escape value, if any; returns 0 and false otherwise.
func (id Escape) CSI() (byte, bool) {
	if 0xEF80 < id && id < 0xEFFF {
		return byte(id & 0x7F), true
	}
	return 0, false
}

// C1Names provides representation names for the C1 extended-ASCII control
// block.
var C1Names = []string{
	"<RES@>",
	"<RESA>",
	"<RESB>",
	"<RESC>",
	"<IND>",
	"<NEL>",
	"<SSA>",
	"<ESA>",
	"<HTS>",
	"<HTJ>",
	"<VTS>",
	"<PLD>",
	"<PLU>",
	"<RI>",
	"<SS2>",
	"<SS3>",
	"<DCS>",
	"<PU1>",
	"<PU2>",
	"<STS>",
	"<CCH>",
	"<MW>",
	"<SPA>",
	"<EPA>",
	"<RESX>",
	"<RESY>",
	"<RESZ>",
	"<CSI>",
	"<ST>",
	"<OSC>",
	"<PM>",
	"<APC>",
}

// String returns a string representation of the identified control, escape
// sequence, or control sequence: C0 controls are represented phonetically, C1
// controls are represented mnemonically, escape sequences are "ESC+b", control
// sequences are "CSI+b", and the two malformed sentinel codepoints are
// "ESC+INVALID" and "CSI+INVALID" respectively. All other codepoints (albeit
// invalid Escape values) are represented using normal "U+XXXX" notation.
func (id Escape) String() string {
	switch {
	case id <= 0x1F:
		return "^" + string(byte(0x40^id))
	case id == 0x7F:
		return "^?"
	case 0x80 <= id && id <= 0x9F:
		return C1Names[id&^0x80]
	case 0xEF20 <= id && id <= 0xEF7E:
		return fmt.Sprintf("ESC+%s", string(byte(id)))
	case 0xEFC0 <= id && id <= 0xEFFE:
		return fmt.Sprintf("CSI+%s", string(byte(0x7f&id)))
	case id == 0xEF7F:
		return "ESC+INVALID"
	case id == 0xEFFF:
		return "CSI+INVALID"
	default:
		return fmt.Sprintf("%U", rune(id))
	}
}

// IsCharacterSetControl returns true if the escape identifier is a character
// control rune, or an character set control escape sequence. Such controls can
// be ignored in a modern UTF-8 terminal.
func (id Escape) IsCharacterSetControl() bool {
	switch id {
	case
		0x000E, // SO     Shift Out, switch to G1 (other half of character set)
		0x000F, // SI     Shift In, switch to G0 (normal half of character set)
		0x008E, // SS2    Single Shift to G2
		0x008F, // SS3    Single Shift to G3 (VT100 uses this for sending PF keys)
		0xEF28, // ESC+(  SCS - Select G0 character set (choice of 63 standard, 16 private)
		0xEF29, // ESC+)  SCS - Select G1 character set (choice of 63 standard, 16 private)
		0xEF2A, // ESC+*  SCS - Select G2 character set
		0xEF2B, // ESC++  SCS - Select G3 character set
		0xEF2C, // ESC+,  SCS - Select G0 character set (additional 63+16 sets)
		0xEF2D, // ESC+-  SCS - Select G1 character set (additional 63+16 sets)
		0xEF2E, // ESC+.  SCS - Select G2 character set
		0xEF2F, // ESC+/  SCS - Select G3 character set
		0xEF6B, // ESC+k  NAPLPS lock-shift G1 to GR
		0xEF6C, // ESC+l  NAPLPS lock-shift G2 to GR
		0xEF6D, // ESC+m  NAPLPS lock-shift G3 to GR
		0xEF6E, // ESC+n  LS2 - Shift G2 to GL (extension of SI) VT240,NAPLPS
		0xEF6F, // ESC+o  LS3 - Shift G3 to GL (extension of SO) VT240,NAPLPS
		0xEF7C, // ESC+|  LS3R - VT240 lock-shift G3 to GR
		0xEF7D, // ESC+}  LS2R - VT240 lock-shift G2 to GR
		0xEF7E: // ESC+~  LS1R - VT240 lock-shift G1 to GR
		return true
	}
	return false
}
