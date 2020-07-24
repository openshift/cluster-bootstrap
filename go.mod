module github.com/openshift/cluster-bootstrap

go 1.13

require (
	github.com/davecgh/go-spew v1.1.1-0.20170626231645-782f4967f2dc
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/gogo/protobuf v0.0.0-20171007142547-342cbe0a0415
	github.com/golang/protobuf v1.1.0
	github.com/google/btree v0.0.0-20160524151835-7d79101e329e
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d
	github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7
	github.com/hashicorp/golang-lru v0.0.0-20160207214719-a0d98a5f2880
	github.com/imdario/mergo v0.3.5
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/json-iterator/go v0.0.0-20180612202835-f2b4162afba3
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/openshift/library-go v0.0.0-20190731171257-950af653b51a
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/spf13/cobra v0.0.2-0.20180319062004-c439c4fa0937
	github.com/spf13/pflag v1.0.1
	golang.org/x/crypto v0.0.0-20180808211826-de0752318171
	golang.org/x/net v0.0.0-20180124060956-0ed95abb35c4
	golang.org/x/oauth2 v0.0.0-20170412232759-a6bd8cefa181
	golang.org/x/sys v0.0.0-20171031081856-95c657629925
	golang.org/x/text v0.0.0-20170810154203-b19bf474d317
	golang.org/x/time v0.0.0-20161028155119-f51c12702a4d
	google.golang.org/appengine v1.6.1
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/yaml.v2 v2.2.1
	k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/klog v0.0.0-20181108234604-8139d8cb77af
	sigs.k8s.io/yaml v1.1.0
)
