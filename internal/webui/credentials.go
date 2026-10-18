package webui

import "xsync/internal/serverstore"

type savedCredential = serverstore.Credential

func savedCredentials() map[string]savedCredential { return serverstore.SavedCredentials() }
