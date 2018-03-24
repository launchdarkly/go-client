package ldclient

type userFilter struct {
	allAttributesPrivate    bool
	globalPrivateAttributes []string
}

func newUserFilter(config Config) userFilter {
	return userFilter{
		allAttributesPrivate:    config.AllAttributesPrivate,
		globalPrivateAttributes: config.PrivateAttributeNames,
	}
}

func (uf userFilter) scrubUser(user User) *User {
	user.PrivateAttributes = nil

	if len(user.PrivateAttributeNames) == 0 && len(uf.globalPrivateAttributes) == 0 && !uf.allAttributesPrivate {
		return &user
	}

	isPrivate := map[string]bool{}
	for _, n := range uf.globalPrivateAttributes {
		isPrivate[n] = true
	}
	for _, n := range user.PrivateAttributeNames {
		isPrivate[n] = true
	}

	if user.Custom != nil {
		var custom = map[string]interface{}{}
		for k, v := range *user.Custom {
			if uf.allAttributesPrivate || isPrivate[k] {
				user.PrivateAttributes = append(user.PrivateAttributes, k)
			} else {
				custom[k] = v
			}
		}
		user.Custom = &custom
	}

	if !isEmpty(user.Avatar) && (uf.allAttributesPrivate || isPrivate["avatar"]) {
		user.Avatar = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "avatar")
	}

	if !isEmpty(user.Country) && (uf.allAttributesPrivate || isPrivate["country"]) {
		user.Country = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "country")
	}

	if !isEmpty(user.Ip) && (uf.allAttributesPrivate || isPrivate["ip"]) {
		user.Ip = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "ip")
	}

	if !isEmpty(user.FirstName) && (uf.allAttributesPrivate || isPrivate["firstName"]) {
		user.FirstName = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "firstName")
	}

	if !isEmpty(user.LastName) && (uf.allAttributesPrivate || isPrivate["lastName"]) {
		user.LastName = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "lastName")
	}

	if !isEmpty(user.Name) && (uf.allAttributesPrivate || isPrivate["name"]) {
		user.Name = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "name")
	}

	if !isEmpty(user.Secondary) && (uf.allAttributesPrivate || isPrivate["secondary"]) {
		user.Secondary = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "secondary")
	}

	if !isEmpty(user.Email) && (uf.allAttributesPrivate || isPrivate["email"]) {
		user.Email = nil
		user.PrivateAttributes = append(user.PrivateAttributes, "email")
	}

	return &user
}

func isEmpty(s *string) bool {
	return s == nil || *s == ""
}
