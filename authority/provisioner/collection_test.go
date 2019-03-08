package provisioner

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/smallstep/assert"
	"github.com/smallstep/cli/jose"
)

func TestCollection_Load(t *testing.T) {
	p, err := generateJWK()
	assert.FatalError(t, err)
	byID := new(sync.Map)
	byID.Store(p.GetID(), p)
	byID.Store("string", "a-string")

	type fields struct {
		byID *sync.Map
	}
	type args struct {
		id string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Interface
		want1  bool
	}{
		{"ok", fields{byID}, args{p.GetID()}, p, true},
		{"fail", fields{byID}, args{"fail"}, nil, false},
		{"invalid", fields{byID}, args{"string"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Collection{
				byID: tt.fields.byID,
			}
			got, got1 := c.Load(tt.args.id)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Collection.Load() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Collection.Load() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCollection_LoadByToken(t *testing.T) {
	p1, err := generateJWK()
	assert.FatalError(t, err)
	p2, err := generateJWK()
	assert.FatalError(t, err)
	p3, err := generateOIDC()
	assert.FatalError(t, err)

	byID := new(sync.Map)
	byID.Store(p1.GetID(), p1)
	byID.Store(p2.GetID(), p2)
	byID.Store(p3.GetID(), p3)
	byID.Store("string", "a-string")

	jwk, err := decryptJSONWebKey(p1.EncryptedKey)
	assert.FatalError(t, err)
	token, err := generateSimpleToken(p1.Name, testAudiences[0], jwk)
	assert.FatalError(t, err)
	t1, c1, err := parseToken(token)
	assert.FatalError(t, err)

	jwk, err = decryptJSONWebKey(p2.EncryptedKey)
	token, err = generateSimpleToken(p2.Name, testAudiences[1], jwk)
	assert.FatalError(t, err)
	t2, c2, err := parseToken(token)
	assert.FatalError(t, err)

	token, err = generateSimpleToken(p3.configuration.Issuer, p3.ClientID, &p3.keyStore.keySet.Keys[0])
	assert.FatalError(t, err)
	t3, c3, err := parseToken(token)
	assert.FatalError(t, err)

	token, err = generateSimpleToken(p3.configuration.Issuer, "string", &p3.keyStore.keySet.Keys[0])
	assert.FatalError(t, err)
	t4, c4, err := parseToken(token)
	assert.FatalError(t, err)

	type fields struct {
		byID      *sync.Map
		audiences []string
	}
	type args struct {
		token  *jose.JSONWebToken
		claims *jose.Claims
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Interface
		want1  bool
	}{
		{"ok1", fields{byID, testAudiences}, args{t1, c1}, p1, true},
		{"ok2", fields{byID, testAudiences}, args{t2, c2}, p2, true},
		{"ok3", fields{byID, testAudiences}, args{t3, c3}, p3, true},
		{"bad", fields{byID, testAudiences}, args{t4, c4}, nil, false},
		{"fail", fields{byID, []string{"https://foo"}}, args{t1, c1}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Collection{
				byID:      tt.fields.byID,
				audiences: tt.fields.audiences,
			}
			got, got1 := c.LoadByToken(tt.args.token, tt.args.claims)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Collection.LoadByToken() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Collection.LoadByToken() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCollection_LoadByCertificate(t *testing.T) {
	p1, err := generateJWK()
	assert.FatalError(t, err)
	p2, err := generateOIDC()
	assert.FatalError(t, err)

	byID := new(sync.Map)
	byID.Store(p1.GetID(), p1)
	byID.Store(p2.GetID(), p2)

	ok1Ext, err := createProvisionerExtension(1, p1.Name, p1.Key.KeyID)
	assert.FatalError(t, err)
	ok2Ext, err := createProvisionerExtension(2, p2.Name, p2.ClientID)
	assert.FatalError(t, err)
	notFoundExt, err := createProvisionerExtension(1, "foo", "bar")
	assert.FatalError(t, err)

	ok1Cert := &x509.Certificate{
		Extensions: []pkix.Extension{ok1Ext},
	}
	ok2Cert := &x509.Certificate{
		Extensions: []pkix.Extension{ok2Ext},
	}
	notFoundCert := &x509.Certificate{
		Extensions: []pkix.Extension{notFoundExt},
	}
	badCert := &x509.Certificate{
		Extensions: []pkix.Extension{
			{Id: stepOIDProvisioner, Critical: false, Value: []byte("foobar")},
		},
	}

	type fields struct {
		byID      *sync.Map
		audiences []string
	}
	type args struct {
		cert *x509.Certificate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Interface
		want1  bool
	}{
		{"ok1", fields{byID, testAudiences}, args{ok1Cert}, p1, true},
		{"ok2", fields{byID, testAudiences}, args{ok2Cert}, p2, true},
		{"noExtension", fields{byID, testAudiences}, args{&x509.Certificate{}}, &noop{}, true},
		{"notFound", fields{byID, testAudiences}, args{notFoundCert}, nil, false},
		{"badCert", fields{byID, testAudiences}, args{badCert}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Collection{
				byID:      tt.fields.byID,
				audiences: tt.fields.audiences,
			}
			got, got1 := c.LoadByCertificate(tt.args.cert)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Collection.LoadByCertificate() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Collection.LoadByCertificate() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCollection_LoadEncryptedKey(t *testing.T) {
	c := NewCollection(testAudiences)
	p1, err := generateJWK()
	assert.FatalError(t, err)
	assert.FatalError(t, c.Store(p1))
	p2, err := generateOIDC()
	assert.FatalError(t, err)
	assert.FatalError(t, c.Store(p2))

	// Add oidc in byKey.
	// It should not happen.
	p2KeyID := p2.keyStore.keySet.Keys[0].KeyID
	c.byKey.Store(p2KeyID, p2)

	type args struct {
		keyID string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{"ok", args{p1.Key.KeyID}, p1.EncryptedKey, true},
		{"oidc", args{p2KeyID}, "", false},
		{"notFound", args{"not-found"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := c.LoadEncryptedKey(tt.args.keyID)
			if got != tt.want {
				t.Errorf("Collection.LoadEncryptedKey() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Collection.LoadEncryptedKey() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCollection_Store(t *testing.T) {
	c := NewCollection(testAudiences)
	p1, err := generateJWK()
	assert.FatalError(t, err)
	p2, err := generateOIDC()
	assert.FatalError(t, err)

	type args struct {
		p Interface
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"ok1", args{p1}, false},
		{"ok2", args{p2}, false},
		{"fail1", args{p1}, true},
		{"fail2", args{p2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := c.Store(tt.args.p); (err != nil) != tt.wantErr {
				t.Errorf("Collection.Store() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollection_Find(t *testing.T) {
	c, err := generateCollection(10, 10)
	assert.FatalError(t, err)

	trim := func(s string) string {
		return strings.TrimLeft(s, "0")
	}
	toList := func(ps provisionerSlice) List {
		l := List{}
		for _, p := range ps {
			l = append(l, p.provisioner)
		}
		return l
	}

	type args struct {
		cursor string
		limit  int
	}
	tests := []struct {
		name  string
		args  args
		want  List
		want1 string
	}{
		{"all", args{"", DefaultProvisionersMax}, toList(c.sorted[0:20]), ""},
		{"0 to 19", args{"", 20}, toList(c.sorted[0:20]), ""},
		{"0 to 9", args{"", 10}, toList(c.sorted[0:10]), trim(c.sorted[10].uid)},
		{"9 to 19", args{trim(c.sorted[10].uid), 10}, toList(c.sorted[10:20]), ""},
		{"1", args{trim(c.sorted[1].uid), 1}, toList(c.sorted[1:2]), trim(c.sorted[2].uid)},
		{"1 to 5", args{trim(c.sorted[1].uid), 4}, toList(c.sorted[1:5]), trim(c.sorted[5].uid)},
		{"defaultLimit", args{"", 0}, toList(c.sorted[0:20]), ""},
		{"overTheLimit", args{"", DefaultProvisionersMax + 1}, toList(c.sorted[0:20]), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := c.Find(tt.args.cursor, tt.args.limit)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Collection.Find() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Collection.Find() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
