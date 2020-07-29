package minio

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/policy"

	"github.com/dulumao/uploader/common"
)

type Client struct {
	*minio.Client
	Config *Config
}

type Config struct {
	AccessID  string
	AccessKey string
	Bucket    string
	Endpoint  string
	Region    string
	IsSSL     bool
	ACL       policy.BucketPolicy
}

func New(config *Config) *Client {
	var (
		err error
		c   = &Client{Config: config}
	)

	Client, err := minio.New(config.Endpoint, config.AccessID, config.AccessKey, config.IsSSL)

	if err == nil {
		c.Client = Client
		//c.Bucket, err = Client.Bucket(config.Bucket)
	}

	if config.ACL == "" {
		config.ACL = policy.BucketPolicyReadWrite
	}
	if err != nil {
		panic(err)
	}

	return c
}

func (c Client) Get(path string) (file *os.File, err error) {
	readCloser, err := c.GetStream(path)

	if err == nil {
		if file, err = ioutil.TempFile("/tmp", "minio"); err == nil {
			defer readCloser.Close()
			_, err = io.Copy(file, readCloser)
			file.Seek(0, 0)
		}
	}

	return file, err
}

func (c Client) GetStream(path string) (io.ReadCloser, error) {
	return c.GetObject(c.Config.Bucket, c.ToRelativePath(path), minio.GetObjectOptions{})
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

func (c Client) Put(urlPath string, reader io.Reader) (*common.Object, error) {
	if seeker, ok := reader.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}

	var err error
	var status bool

	status, err = c.BucketExists(c.Config.Bucket)

	if err == nil && !status {
		// instead of storing in environment variable, if possible
		// use other way depending on your architecture
		if err := c.MakeBucket(c.Config.Bucket, c.Config.Region); err != nil {
			return nil, err
		}

		//policy := minio.NewPostPolicy()
		// Apply upload policy restrictions:
		//policy.SetBucket(c.Config.Bucket)
		//policy.String()

		//policy := `{"Version": "2012-10-17","Statement": [{"Action": ["s3:GetObject"],"Effect": "Allow","Principal": {"AWS": ["*"]},"Resource": ["arn:aws:s3:::my-bucketname/*"],"Sid": ""}]}`

		// 可选值有[PolicyType.NONE, PolicyType.READ_ONLY, PolicyType.READ_WRITE, PolicyType.WRITE_ONLY].
		//if err := c.SetBucketPolicy(c.Config.Bucket, "READ_WRITE"); err != nil {
		//	panic(err)
		//	return nil, err
		//}
	}

	extension := path.Ext(urlPath)
	fileMIME := filetype.GetType(extension[1:])
	contentType := fileMIME.MIME.Value

	_, err = c.PutObject(c.Config.Bucket, c.ToRelativePath(urlPath), reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})

	if err != nil {
		panic(err)
	}

	now := time.Now()

	return &common.Object{
		Path:         urlPath,
		Name:         filepath.Base(urlPath),
		LastModified: &now,
	}, err
}

func (c Client) Delete(path string) error {
	return c.RemoveObject(c.Config.Bucket, c.ToRelativePath(path))
}

func (c Client) List(path string) ([]*common.Object, error) {
	var objects []*common.Object
	var doneCh = make(chan struct{})
	defer close(doneCh)

	for obj := range c.ListObjectsV2(c.Config.Bucket, path, false, doneCh) {
		objects = append(objects, &common.Object{
			Path:         "/" + c.ToRelativePath(obj.Key),
			Name:         filepath.Base(obj.Key),
			LastModified: &obj.LastModified,
		})
	}

	return objects, nil
}

func (c Client) GetEndpoint() string {
	if c.Config.IsSSL {
		return "https://" + c.Config.Endpoint + "/" + c.Config.Bucket
	}

	return "http://" + c.Config.Endpoint + "/" + c.Config.Bucket
}

func (c Client) GetURL(path string) (string, error) {
	var url = c.GetEndpoint() + "/" + path

	return url, nil
}
