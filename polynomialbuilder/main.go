package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	secp "github.com/btcsuite/btcd/btcec"
	poly "github.com/jongukim/polynomial"
	"golang.org/x/crypto/sha3"
	"io/ioutil"
	//	"math"
	"math/big"
	"strconv"
)

// S is KoblitzCurve from btcec
var S *secp.KoblitzCurve
var group = secp.S256()
var grouporder = group.N
var H, _ = HashToCurve([]byte("i am a stupid moron"))

// we need a curve point type so that curve points are just one thing
// as opposed to being representing by their bigint affine coordinates x, y :)

// CurvePoint lets us use the bigint affine point rep as one var not two :)
type CurvePoint struct {
	X *big.Int `json:"x"`
	Y *big.Int `json:"y"`
}

// String takes a CurvePoint and converts to a string for pretty printing (& interfacing?)
func (c CurvePoint) String() string {
	return fmt.Sprintf("X: %s, Y: %s", c.X, c.Y)
}

// ScalarBaseMult lets us do g.mult(scalar) from btcec with CurvePoints rather than (x, y)
func (c CurvePoint) ScalarBaseMult(x *big.Int) CurvePoint {
	px, py := S.ScalarBaseMult(x.Bytes())
	return CurvePoint{px, py}
}

// ScalarMult lets us do point.mult(scalar) from btcec with CurvePoints rather than (x, y)
func (c CurvePoint) ScalarMult(x *big.Int) CurvePoint {
	px, py := S.ScalarMult(c.X, c.Y, x.Bytes())
	return CurvePoint{px, py}
}

// Add lets us do point1.Add(point2) from btcec with CurvePoints rather than (x, y)
func (c CurvePoint) Add(y CurvePoint) CurvePoint {
	px, py := S.Add(c.X, c.Y, y.X, y.Y)
	return CurvePoint{px, py}
}

// PrivKeysStr is an array of the private keys as strings
type PrivKeysStr struct {
	Keys []string `json:"privkeys"`
}

// PubKeyStr is a single public key, represented by its affine coords (as strings)
type PubKeyStr struct {
	X string `json:"x"`
	Y string `json:"y"`
}

// RingStr is an array of PubKeyStrs, which are pubkeys as strings
type RingStr struct {
	PubKeys []PubKeyStr `json:"pubkeys"`
}

// PubKey = CurvePoint = affine, bigint representation of EC points
type PubKey struct {
	CurvePoint
}

// Ring is an array of PubKeys (bigint EC point coords). Acts as a list of public keys.
type Ring struct {
	PubKeys []PubKey `json:"pubkeys"`
}

func main() {
	// read in all the private keys
	privkeyfile, err := ioutil.ReadFile("privkeys.json")
	sk := PrivKeysStr{} // because all json files are read in as strings
	if err = json.Unmarshal(privkeyfile, &sk); err != nil {
		panic(err) // we should do better error handling lol
	}

	// read in all the public keys
	keyfile, _ := ioutil.ReadFile("pubkeys.json")
	pk := RingStr{}
	if err = json.Unmarshal(keyfile, &pk); err != nil {
		panic(err)
	}
	pubkeys := ConvertPubKeys(pk)
	fmt.Println(pubkeys)

	// now we unwrap all the private keys
	for i := 0; i < len(sk.Keys); i++ {
		privbytes, err := hex.DecodeString(sk.Keys[i])
		if err != nil {
			panic(err)
		}
		privbn := new(big.Int).SetBytes(privbytes)
		fmt.Println(privbn)

	}
	// len(sk.Keys) is a silly hacky way of getting the ring size.
	// it should defs be changed irl
	var polyarray []poly.Poly
	for i := 0; i < len(sk.Keys); i++ {
		randompoly := PolynomialBuilder(int(3), len(sk.Keys), int(i))
		// we build polyarray like p[0][k], p[1][k], ...
		polyarray = append(polyarray, randompoly)
	}
	fmt.Println(polyarray)

}

/*


func Mint() CurvePoint, *big.Int, *big.Int {
	privkey, e := rand.Int(rand.Reader, grouporder)
	check(e)
	serial, e := rand.Int(rand.Reader, grouporder)
	check(e)
	c := commit(serial, privkey)
	return c, privkey, serial
}

func Spend(pp, M, c, C) *big.Int {
}

func Verify(pp, M, serial, C, pi) {
}


*/

func Commit(a *big.Int, b *big.Int) CurvePoint {
	ha := H.ScalarMult(a)
	gb := CurvePoint{}.ScalarBaseMult(b)
	return ha.Add(gb)
}

func HashToCurve(s []byte) (CurvePoint, error) {
	q := group.P
	x := big.NewInt(0)
	y := big.NewInt(0)
	z := big.NewInt(0)
	// what is this magical number
	z.SetString("57896044618658097711785492504343953926634992332820282019728792003954417335832", 10)

	// sum256 outputs an array of 32 bytes :) => are we menna use   keccak? does this work?
	array := sha3.Sum256(s)
	x = Convert(array[:])
	for true {
		xcubed := new(big.Int).Exp(x, big.NewInt(3), q)
		xcubed7 := new(big.Int).Add(xcubed, big.NewInt(7))
		y.ModSqrt(xcubed7, q)
		y.Set(q)
		y.Add(y, big.NewInt(1))
		y.Rsh(y, 2)
		y.Exp(xcubed7, y, q)
		z = z.Exp(y, big.NewInt(2), q)
		posspoint := S.IsOnCurve(x, y)
		if posspoint == true {
			return CurvePoint{x, y}, nil
		}
		x.Add(x, big.NewInt(1))
	}
	return CurvePoint{}, errors.New("no curve point found")
}

func PolynomialBuilder(signerindex int, ringsize int, i int) poly.Poly {

	// this is just to print and get the bit length, n
	// signerindexbin := strconv.FormatInt(int64(signerindex), 2)
	ringbin := strconv.FormatInt(int64(ringsize), 2)
	var product poly.Poly
	// the products of functions defined by each i form distinct polynomials (one per i)
	// this polynomial will have degree max bitlength(ringlength)

	// things need to be uint so the bitshifting works
	// len(ringbin) = n
	// ------------------------------------------------------------------------------
	// is it gonna cause problems that we're running 0 -> n - 1 rather than 1 -> n :(

	// j is the bit index.
	// the functions defined in this bit get multiplied together to form the poly above
	for j := uint(0); j < uint(len(ringbin)); j++ {

		var functiontemp poly.Poly
		aj, e := rand.Int(rand.Reader, grouporder)
		Check(e)
		z, e := rand.Int(rand.Reader, grouporder)
		Check(e)

		// we compare i (the current index) to l (the signer index), bitwise
		if (i >> j & 0x1) == 0 {
			if ((signerindex >> j) & 0x1) == 0 {
				// f = x - aj
				functiontemp = append(functiontemp, z.ModInverse(aj, grouporder))
				functiontemp = append(functiontemp, big.NewInt(1))
			}
			if ((signerindex >> j) & 0x1) == 1 {
				// otherwise it's just - aj
				functiontemp = append(functiontemp, z.ModInverse(aj, grouporder))
				functiontemp = append(functiontemp, big.NewInt(0))
			}
		}

		if (i >> j & 0x1) == 1 {
			if ((signerindex >> j) & 0x1) == 1 {
				// f = x + aj
				// this mod is super redundant
				functiontemp = append(functiontemp, z.Mod(aj, grouporder))
				functiontemp = append(functiontemp, big.NewInt(1))
			}
			if ((signerindex >> j) & 0x1) == 0 {
				// otherwise it's just aj
				// this mod is super redundant
				functiontemp = append(functiontemp, z.Mod(aj, grouporder))
				functiontemp = append(functiontemp, big.NewInt(0))
			}
		}

		if j == 0 {
			// i should do this in some prettier way hey?
			product = poly.NewPolyInts(0, 0, 0, 0, 0)
			product = functiontemp
		} else {
			product = product.Mul(functiontemp, grouporder)
		}
	}
	return product
}

func ConvertPubKeys(rn RingStr) Ring {

	rl := len(rn.PubKeys)
	//fmt.Println("Length : ", rl)
	var ring Ring

	for i := 0; i < rl; i++ {
		var bytesx []byte
		var bytesy []byte
		bytesx, _ = hex.DecodeString(string(rn.PubKeys[i].X))
		bytesy, _ = hex.DecodeString(string(rn.PubKeys[i].Y))
		pubkeyx := new(big.Int).SetBytes(bytesx) // This makes big int
		pubkeyy := new(big.Int).SetBytes(bytesy) // So we can do EC arithmetic
		ring.PubKeys = append(ring.PubKeys, PubKey{CurvePoint{pubkeyx, pubkeyy}})
	}
	return ring
}

func Convert(data []byte) *big.Int {
	z := new(big.Int)
	z.SetBytes(data)
	return z
}

func Check(e error) {
	if e != nil {
		panic(e)
	}
}
