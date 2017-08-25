package app

func toStringMap(v map[string]interface{}) (map[string]string, bool) {
	if len(v) == 0 {
		return nil, true
	}

	vs := make(map[string]string, len(v))
	for _k, _v := range v {
		s, ok := _v.(string)
		if !ok {
			return nil, false
		}
		vs[_k] = s
	}
	return vs, true
}
