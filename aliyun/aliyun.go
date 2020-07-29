package aliyun

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	aliyun "github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/dulumao/uploader/common"
)

type Client struct {
	*aliyun.Bucket
	Config *Config
}

type Config struct {
	AccessID      string
	AccessKey     string
	Region        string
	Bucket        string
	Endpoint      string
	ACL           aliyun.ACLType
	ClientOptions []aliyun.ClientOption
	UseCname      bool
}

func New(config *Config) *Client {
	var (
		err error
		c   = &Client{Config: config}
	)

	if config.Endpoint == "" {
		config.Endpoint = "oss-cn-hangzhou.aliyuncs.com"
	}

	if config.ACL == "" {
		config.ACL = aliyun.ACLPublicRead
	}

	if config.UseCname {
		config.ClientOptions = append(config.ClientOptions, aliyun.UseCname(config.UseCname))
	}

	Aliyun, err := aliyun.New(config.Endpoint, config.AccessID, config.AccessKey, config.ClientOptions...)

	if err == nil {
		c.Bucket, err = Aliyun.Bucket(config.Bucket)
	}

	if err != nil {
		panic(err)
	}

	return c
}

func (c Client) Get(path string) (file *os.File, err error) {
	readCloser, err := c.GetStream(path)

	if err == nil {
		if file, err = ioutil.TempFile("/tmp", "ali"); err == nil {
			defer readCloser.Close()
			_, err = io.Copy(file, readCloser)
			file.Seek(0, 0)
		}
	}

	return file, err
}

func (c Client) GetStream(path string) (io.ReadCloser, error) {
	return c.Bucket.GetObject(c.ToRelativePath(path))
}

var urlRegexp = regexp.MustCompile(`(https?:)?//((\w+).)+(\w+)/`)

// ToRelativePath process path to relative path
func (c Client) ToRelativePath(urlPath string) string {
	if urlRegexp.MatchString(urlPath) {
		if u, err := url.Parse(urlPath); err == nil {
			return strings.TrimPrefix(u.Path, "/")
		}
	}

	return strings.TrimPrefix(urlPath, "/")
}

func (client Client) Put(urlPath string, reader io.Reader) (*common.Object, error) {
	if seeker, ok := reader.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}

	err := client.Bucket.PutObject(client.ToRelativePath(urlPath), reader, aliyun.ACL(client.Config.ACL))
	now := time.Now()

	return &common.Object{
		Path:         urlPath,
		Name:         filepath.Base(urlPath),
		LastModified: &now,
	}, err
}

func (c Client) Delete(path string) error {
	return c.Bucket.DeleteObject(c.ToRelativePath(path))
}

func (c Client) List(path string) ([]*common.Object, error) {
	var objects []*common.Object

	results, err := c.Bucket.ListObjects(aliyun.Prefix(path))

	if err == nil {
		for _, obj := range results.Objects {
			objects = append(objects, &common.Object{
				Path:         "/" + c.ToRelativePath(obj.Key),
				Name:         filepath.Base(obj.Key),
				LastModified: &obj.LastModified,
			})
		}
	}

	return objects, err
}

func (c Client) GetEndpoint() string {
	if c.Config.Endpoint != "" {
		if strings.HasSuffix(c.Config.Endpoint, "aliyuncs.com") {
			return c.Config.Bucket + "." + c.Config.Endpoint
		}
		return c.Config.Endpoint
	}

	endpoint := c.Bucket.Client.Config.Endpoint
	for _, prefix := range []string{"https://", "http://"} {
		endpoint = strings.TrimPrefix(endpoint, prefix)
	}

	return c.Config.Bucket + "." + endpoint
}

func (c Client) GetURL(path string) (url string, err error) {
	if c.Config.ACL == aliyun.ACLPrivate {
		return c.Bucket.SignURL(c.ToRelativePath(path), aliyun.HTTPGet, 60*60) // 1 hour
	}
	return path, nil
}
