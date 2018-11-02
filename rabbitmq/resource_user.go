package rabbitmq

import (
	"fmt"
	"log"
	"strings"

	"github.com/michaelklishin/rabbit-hole"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Create: CreateUser,
		Update: UpdateUser,
		Read:   ReadUser,
		Delete: DeleteUser,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"password": &schema.Schema{
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},

			"tags": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func CreateUser(d *schema.ResourceData, meta interface{}) error {
	rmqc := meta.(*rabbithole.Client)

	name := d.Get("name").(string)

	userSettings := rabbithole.UserSettings{
		Password: d.Get("password").(string),
		Tags:     userTagsToString(d),
	}

	log.Printf("[DEBUG] RabbitMQ: Attempting to create user %s", name)

	return resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		resp, err := rmqc.PutUser(name, userSettings)
		log.Printf("[DEBUG] RabbitMQ: user creation response: %#v", resp)
		if err != nil {
			return resource.RetryableError(fmt.Errorf("expected user to be created but received error: %s", err))
		}

		d.SetId(name)
		return resource.NonRetryableError(ReadUser(d, meta))
	})
}

func ReadUser(d *schema.ResourceData, meta interface{}) error {
	rmqc := meta.(*rabbithole.Client)

	user, err := rmqc.GetUser(d.Id())
	if err != nil {
		return checkDeleted(d, err)
	}

	log.Printf("[DEBUG] RabbitMQ: User retrieved: %#v", user)

	d.Set("name", user.Name)

	if len(user.Tags) > 0 {
		tags := strings.Split(user.Tags, ",")
		d.Set("tags", tags)
	}

	return nil
}

func UpdateUser(d *schema.ResourceData, meta interface{}) error {
	rmqc := meta.(*rabbithole.Client)

	name := d.Id()

	if d.HasChange("password") {
		tags := userTagsToString(d)
		password := d.Get("password").(string)

		userSettings := rabbithole.UserSettings{
			Password: password,
			Tags:     tags,
		}

		log.Printf("[DEBUG] RabbitMQ: Attempting to update password for %s", name)

		resp, err := rmqc.PutUser(name, userSettings)
		log.Printf("[DEBUG] RabbitMQ: Password update response: %#v", resp)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("Error updating RabbitMQ user: %s", resp.Status)
		}

	}

	if d.HasChange("tags") {
		userSettings := rabbithole.UserSettings{}
		userSettings.Tags = userTagsToString(d)

		log.Printf("[DEBUG] RabbitMQ: Attempting to update tags for %s", name)

		resp, err := rmqc.PutUser(name, userSettings)
		log.Printf("[DEBUG] RabbitMQ: Tags update response: %#v", resp)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("Error updating RabbitMQ user: %s", resp.Status)
		}

	}

	return ReadUser(d, meta)
}

func DeleteUser(d *schema.ResourceData, meta interface{}) error {
	rmqc := meta.(*rabbithole.Client)

	name := d.Id()
	log.Printf("[DEBUG] RabbitMQ: Attempting to delete user %s", name)

	resp, err := rmqc.DeleteUser(name)
	log.Printf("[DEBUG] RabbitMQ: User delete response: %#v", resp)
	if err != nil {
		return err
	}

	if resp.StatusCode == 404 {
		// the user was automatically deleted
		return nil
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Error deleting RabbitMQ user: %s", resp.Status)
	}

	return nil
}

func userTagsToString(d *schema.ResourceData) string {
	var tags string
	tagList := []string{}
	for _, v := range d.Get("tags").([]interface{}) {
		if tag, ok := v.(string); ok {
			tagList = append(tagList, tag)
		}
	}
	tags = strings.Join(tagList, ",")

	return tags
}
