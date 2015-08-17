package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/dt/thile/gen"
	"github.com/foursquare/gohfile"
)

type ThriftRpcImpl struct {
	*hfile.CollectionSet
}

func (cs *ThriftRpcImpl) GetValuesSingle(req *gen.SingleHFileKeyRequest) (r *gen.SingleHFileKeyResponse, err error) {
	if Settings.debug {
		log.Printf("[GetValuesSingle] %s (%d keys)\n", *req.HfileName, len(req.SortedKeys))
	}
	reader, err := cs.ScannerFor(*req.HfileName)
	if err != nil {
		return nil, err
	}

	if req.PerKeyValueLimit != nil {
		// TODO(davidt) impl
		log.Println("[GetValuesSingle] PerKeyValueLimit. oh well.")
	}

	if req.CountOnly != nil {
		// TODO(davidt) impl
		log.Println("[GetValuesSingle] CountOnly. oh well.")
	}

	res := new(gen.SingleHFileKeyResponse)
	res.Values = make(map[int32][]byte)
	found := int32(0)

	for idx, key := range req.SortedKeys {
		if Settings.debug {
			log.Printf("[GetValuesSingle] key: %s\n", hex.EncodeToString(key))
		}
		value, err, ok := reader.GetFirst(key)
		if err != nil {
			return nil, err
		}
		if ok {
			found++
			res.Values[int32(idx)] = value
		}
	}

	if Settings.debug {
		log.Printf("[GetValuesSingle] %s found %d of %d.\n", *req.HfileName, found, len(req.SortedKeys))
	}
	res.KeyCount = &found
	return res, nil
}

func (cs *ThriftRpcImpl) GetValuesMulti(req *gen.SingleHFileKeyRequest) (r *gen.MultiHFileKeyResponse, err error) {
	if Settings.debug {
		log.Println("[GetValuesMulti]", len(req.SortedKeys))
	}

	reader, err := cs.ScannerFor(*req.HfileName)
	if err != nil {
		return nil, err
	}

	res := new(gen.MultiHFileKeyResponse)
	found := int32(0)

	for idx, key := range req.SortedKeys {
		values, err := reader.GetAll(key)
		if err != nil {
			return nil, err
		}
		if len(values) > 0 {
			found += int32(len(values))
			res.Values[int32(idx)] = values
		}
	}

	res.KeyCount = &found
	return res, nil

}

func (cs *ThriftRpcImpl) GetValuesForPrefixes(req *gen.PrefixRequest) (r *gen.PrefixResponse, err error) {
	res := new(gen.PrefixResponse)
	if reader, err := cs.ReaderFor(*req.HfileName); err != nil {
		return nil, err
	} else {
		i := reader.NewIterator()
		if res.Values, err = i.AllForPrfixes(req.SortedKeys); err != nil {
			return nil, err
		} else {
			return res, nil
		}
	}
}

func (cs *ThriftRpcImpl) GetValuesMultiSplitKeys(req *gen.MultiHFileSplitKeyRequest) (r *gen.KeyToValuesResponse, err error) {
	res := make(map[string][][]byte)
	scanner, err := cs.ScannerFor(*req.HfileName)
	if err != nil {
		return nil, err
	}

	for _, parts := range RevProduct(req.SplitKey) {
		// TODO(davidt): avoid allocing concated key by adding split-key search lower down.
		key := bytes.Join(parts, nil)

		if values, err := scanner.GetAll(key); err != nil {
			return nil, err
		} else if len(values) > 0 {
			res[string(key)] = values
		}
	}
	return &gen.KeyToValuesResponse{res}, nil
}

func (cs *ThriftRpcImpl) GetIterator(req *gen.IteratorRequest) (*gen.IteratorResponse, error) {
	// 	HfileName     *string `thrift:"hfileName,1" json:"hfileName"`
	// 	IncludeValues *bool   `thrift:"includeValues,2" json:"includeValues"`
	// 	LastKey       []byte  `thrift:"lastKey,3" json:"lastKey"`
	// 	SkipKeys      *int32  `thrift:"skipKeys,4" json:"skipKeys"`
	// 	ResponseLimit *int32  `thrift:"responseLimit,5" json:"responseLimit"`
	// 	EndKey        []byte  `thrift:"endKey,6" json:"endKey"`
	var err error

	if req.ResponseLimit == nil {
		return nil, fmt.Errorf("Missing limit.")
	}
	limit := int(*req.ResponseLimit)

	reader, err := cs.ReaderFor(*req.HfileName)
	if err != nil {
		return nil, err
	}
	it := reader.NewIterator()

	remaining := false

	if req.LastKey != nil {
		remaining, err = it.Seek(req.LastKey)
	} else {
		remaining, err = it.Next()
	}

	if err != nil {
		return nil, err
	}

	res := new(gen.IteratorResponse)

	if !remaining {
		return res, nil
	}

	skipKeys := int32(0)
	lastKey := it.Key()

	if toSkip := req.GetSkipKeys(); toSkip > 0 {
		for i := int32(0); i < toSkip && remaining; i++ {
			if bytes.Equal(lastKey, it.Key()) {
				skipKeys = skipKeys + 1
			} else {
				skipKeys = 0
			}

			lastKey = it.Key()

			remaining, err = it.Next()
			if err != nil {
				return nil, err
			}
		}
		if !remaining {
			return res, nil
		}
	}

	if req.EndKey != nil {
		remaining = remaining && !hfile.After(it.Key(), req.EndKey)
	}

	r := make([]*gen.KeyValueItem, 0)
	for i := 0; i < limit && remaining; i++ {
		v := []byte{}
		if req.IncludeValues == nil || *req.IncludeValues {
			v = it.Value()
		}
		r = append(r, &gen.KeyValueItem{it.Key(), v})

		if bytes.Equal(lastKey, it.Key()) {
			skipKeys = skipKeys + 1
		} else {
			skipKeys = 0
		}
		lastKey = it.Key()

		remaining, err = it.Next()
		if err != nil {
			return nil, err
		}
		if req.EndKey != nil {
			remaining = remaining && !hfile.After(it.Key(), req.EndKey)
		}
	}
	return &gen.IteratorResponse{r, lastKey, &skipKeys}, nil
}

func (cs *ThriftRpcImpl) GetInfo(req *gen.InfoRequest) (r []*gen.HFileInfo, err error) {
	return nil, fmt.Errorf("Not implemented")
}

func (cs *ThriftRpcImpl) TestTimeout(waitInMillis int32) (r int32, err error) {
	return 0, fmt.Errorf("Not implemented")
}
