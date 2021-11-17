// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by consensys/gnark-crypto DO NOT EDIT

package plookup

import (
	"crypto/sha256"
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bw6-761"
	"github.com/consensys/gnark-crypto/ecc/bw6-761/fr"
	"github.com/consensys/gnark-crypto/ecc/bw6-761/fr/fft"
	"github.com/consensys/gnark-crypto/ecc/bw6-761/fr/kzg"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
)

var (
	ErrIncompatibleSize = errors.New("the tables in f and t are not of the same size")
	ErrFoldedCommitment = errors.New("the folded commitment is malformed")
)

// ProofLookupTables proofs that a list of tables
type ProofLookupTables struct {

	// commitments to the rows f
	fs []kzg.Digest

	// lookup proof for the f and t folded
	foldedProof ProofLookupVector
}

// ProveLookupTables generates a proof that f, seen as a multi dimensional table,
// consists of vectors that are in t. In other words for each i, f[:][i] must be one
// of the t[:][j].
//
// For instance, if t is the truth table of the XOR function, t will be populated such
// that t[:][i] contains the i-th entry of the truth table, so t[0][i] XOR t[1][i] = t[2][i].
//
// The Table in f and t are supposed to be of the same size constant size.
func ProveLookupTables(srs *kzg.SRS, f, t []Table) (ProofLookupTables, error) {

	// res
	proof := ProofLookupTables{}
	var err error

	// hash function used for Fiat Shamir
	hFunc := sha256.New()

	// transcript to derive the challenge
	fs := fiatshamir.NewTranscript(hFunc, "lambda")

	// check the sizes
	if len(f) != len(t) {
		return proof, ErrIncompatibleSize
	}
	s := len(f[0])
	for i := 1; i < len(f); i++ {
		if len(f[i]) != s {
			return proof, ErrIncompatibleSize
		}
	}
	s = len(t[0])
	for i := 1; i < len(t); i++ {
		if len(t[i]) != s {
			return proof, ErrIncompatibleSize
		}
	}

	// commit to the tables in f and t
	nbRows := len(t)
	proof.fs = make([]kzg.Digest, nbRows)
	_nbColumns := len(f[0]) + 1
	if _nbColumns < len(t[0]) {
		_nbColumns = len(t[0])
	}
	d := fft.NewDomain(uint64(_nbColumns), 0, false)
	nbColumns := d.Cardinality
	lfs := make([][]fr.Element, nbRows)
	cfs := make([][]fr.Element, nbRows)
	lts := make([][]fr.Element, nbRows)

	for i := 0; i < nbRows; i++ {

		cfs[i] = make([]fr.Element, nbColumns)
		lfs[i] = make([]fr.Element, nbColumns)
		copy(cfs[i], f[i])
		copy(lfs[i], f[i])
		for j := len(f[i]); j < int(nbColumns); j++ {
			cfs[i][j] = f[i][len(f[i])-1]
			lfs[i][j] = f[i][len(f[i])-1]
		}
		d.FFTInverse(cfs[i], fft.DIF, 0)
		fft.BitReverse(cfs[i])
		proof.fs[i], err = kzg.Commit(cfs[i], srs)
		if err != nil {
			return proof, err
		}

		lts[i] = make([]fr.Element, d.Cardinality)
		copy(lts[i], t[i])
		for j := len(t[i]); j < int(d.Cardinality); j++ {
			lts[i][j] = t[i][len(t[i])-1]
		}
	}

	// fold f and t
	comms := make([]*kzg.Digest, nbRows)
	for i := 0; i < nbRows; i++ {
		comms[i] = new(kzg.Digest)
		comms[i].Set(&proof.fs[i])
	}
	lambda, err := deriveRandomness(&fs, "lambda", comms...)
	if err != nil {
		return proof, err
	}
	foldedf := make(Table, nbColumns)
	foldedt := make(Table, nbColumns)
	for i := 0; i < int(nbColumns); i++ {
		for j := nbRows - 1; j >= 0; j-- {
			foldedf[i].Mul(&foldedf[i], &lambda).
				Add(&foldedf[i], &lfs[j][i])
			foldedt[i].Mul(&foldedt[i], &lambda).
				Add(&foldedt[i], &lts[j][i])
		}
	}

	// call plookupVector, on foldedf[:len(foldedf)-1] to ensure that the domain size
	// in ProveLookupVector is the same as d's
	proof.foldedProof, err = ProveLookupVector(srs, foldedf[:len(foldedf)-1], foldedt)

	return proof, err
}

// VerifyLookupTables verifies that a ProofLookupTables proof is correct.
func VerifyLookupTables(srs *kzg.SRS, proof ProofLookupTables) error {

	// hash function used for Fiat Shamir
	hFunc := sha256.New()

	// transcript to derive the challenge
	fs := fiatshamir.NewTranscript(hFunc, "lambda")

	// fold the commitments
	nbRows := len(proof.fs)
	comms := make([]*kzg.Digest, nbRows)
	for i := 0; i < nbRows; i++ {
		comms[i] = &proof.fs[i]
	}
	lambda, err := deriveRandomness(&fs, "lambda", comms...)
	if err != nil {
		return err
	}

	// verify that the commitments in the inner proof are consistant
	// with the folded commitments.
	var comf kzg.Digest
	comf.Set(&proof.fs[nbRows-1])
	var blambda big.Int
	lambda.ToBigIntRegular(&blambda)
	for i := nbRows - 2; i >= 0; i-- {
		comf.ScalarMultiplication(&comf, &blambda).
			Add(&comf, &proof.fs[i])
	}

	if !comf.Equal(&proof.foldedProof.f) {
		return ErrFoldedCommitment
	}

	// verify the inner proof
	return VerifyLookupVector(srs, proof.foldedProof)
}

// TODO put that in fiat-shamir package
func deriveRandomness(fs *fiatshamir.Transcript, challenge string, points ...*bw6761.G1Affine) (fr.Element, error) {

	var buf [bw6761.SizeOfG1AffineUncompressed]byte
	var r fr.Element

	for _, p := range points {
		buf = p.RawBytes()
		if err := fs.Bind(challenge, buf[:]); err != nil {
			return r, err
		}
	}

	b, err := fs.ComputeChallenge(challenge)
	if err != nil {
		return r, err
	}
	r.SetBytes(b)
	return r, nil
}
