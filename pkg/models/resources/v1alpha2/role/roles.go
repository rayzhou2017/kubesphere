/*

 Copyright 2019 The KubeSphere Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.

*/
package role

import (
	"k8s.io/client-go/informers"
	"kubesphere.io/kubesphere/pkg/constants"
	"kubesphere.io/kubesphere/pkg/models/resources/v1alpha2"

	"kubesphere.io/kubesphere/pkg/server/params"
	"kubesphere.io/kubesphere/pkg/utils/sliceutil"
	"sort"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type roleSearcher struct {
	informers informers.SharedInformerFactory
}

func NewRoleSearcher(informers informers.SharedInformerFactory) v1alpha2.Interface {
	return &roleSearcher{informers: informers}
}

func (s *roleSearcher) Get(namespace, name string) (interface{}, error) {
	return s.informers.Rbac().V1().Roles().Lister().Roles(namespace).Get(name)
}

// exactly Match
func (*roleSearcher) match(match map[string]string, item *rbac.Role) bool {
	for k, v := range match {
		switch k {
		case v1alpha2.Name:
			names := strings.Split(v, "|")
			if !sliceutil.HasString(names, item.Name) {
				return false
			}
		case v1alpha2.Keyword:
			if !strings.Contains(item.Name, v) && !v1alpha2.SearchFuzzy(item.Labels, "", v) && !v1alpha2.SearchFuzzy(item.Annotations, "", v) {
				return false
			}
		case v1alpha2.UserFacing:
			if v == "true" {
				if !isUserFacingRole(item) {
					return false
				}
			}
		default:
			// label not exist or value not equal
			if val, ok := item.Labels[k]; !ok || val != v {
				return false
			}
		}
	}
	return true
}

// Fuzzy searchInNamespace
func (*roleSearcher) fuzzy(fuzzy map[string]string, item *rbac.Role) bool {
	for k, v := range fuzzy {
		switch k {
		case v1alpha2.Name:
			if !strings.Contains(item.Name, v) && !strings.Contains(item.Annotations[constants.DisplayNameAnnotationKey], v) {
				return false
			}
		case v1alpha2.Label:
			if !v1alpha2.SearchFuzzy(item.Labels, "", v) {
				return false
			}
		case v1alpha2.Annotation:
			if !v1alpha2.SearchFuzzy(item.Annotations, "", v) {
				return false
			}
			return false
		default:
			if !v1alpha2.SearchFuzzy(item.Labels, k, v) {
				return false
			}
		}
	}
	return true
}

func (*roleSearcher) compare(a, b *rbac.Role, orderBy string) bool {
	switch orderBy {
	case v1alpha2.CreateTime:
		return a.CreationTimestamp.Time.Before(b.CreationTimestamp.Time)
	case v1alpha2.Name:
		fallthrough
	default:
		return strings.Compare(a.Name, b.Name) <= 0
	}
}

func (s *roleSearcher) Search(namespace string, conditions *params.Conditions, orderBy string, reverse bool) ([]interface{}, error) {
	roles, err := s.informers.Rbac().V1().Roles().Lister().Roles(namespace).List(labels.Everything())

	if err != nil {
		return nil, err
	}

	result := make([]*rbac.Role, 0)

	if len(conditions.Match) == 0 && len(conditions.Fuzzy) == 0 {
		result = roles
	} else {
		for _, item := range roles {
			if s.match(conditions.Match, item) && s.fuzzy(conditions.Fuzzy, item) {
				result = append(result, item)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if reverse {
			tmp := i
			i = j
			j = tmp
		}
		return s.compare(result[i], result[j], orderBy)
	})

	r := make([]interface{}, 0)
	for _, i := range result {
		r = append(r, i)
	}
	return r, nil
}

// role created by user from kubesphere dashboard
func isUserFacingRole(role *rbac.Role) bool {
	if role.Annotations[constants.CreatorAnnotationKey] != "" {
		return true
	}
	return false
}