package analysisservices

import (
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/analysisservices/mgmt/2017-08-01/analysisservices"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/clients"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/features"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/analysisservices/parse"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/analysisservices/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tags"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/p"
	azSchema "github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmAnalysisServicesServer() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmAnalysisServicesServerCreate,
		Read:   resourceArmAnalysisServicesServerRead,
		Update: resourceArmAnalysisServicesServerUpdate,
		Delete: resourceArmAnalysisServicesServerDelete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Importer: azSchema.ValidateResourceIDPriorToImport(func(id string) error {
			_, err := parse.AnalysisServicesServerID(id)
			return err
		}),

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.AnalysisServicesServerName,
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"location": azure.SchemaLocation(),

			"sku": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"D1",
					"B1",
					"B2",
					"S0",
					"S1",
					"S2",
					"S4",
					"S8",
					"S9",
				}, false),
			},

			"admin_users": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"enable_power_bi_service": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"ipv4_firewall_rule": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"range_start": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.IsIPv4Address,
						},
						"range_end": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.IsIPv4Address,
						},
					},
				},
			},

			"querypool_connection_mode": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validate.QuerypoolConnectionMode(),
			},

			"backup_blob_container_uri": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"server_full_name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tags": tags.Schema(),
		},
	}
}

func resourceArmAnalysisServicesServerCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AnalysisServices.ServerClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for Azure ARM Analysis Services Server creation.")

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	if features.ShouldResourcesBeImported() && d.IsNewResource() {
		server, err := client.GetDetails(ctx, resourceGroup, name)
		if err != nil {
			if !utils.ResponseWasNotFound(server.Response) {
				return fmt.Errorf("Error checking for presence of existing Analysis Services Server %q (Resource Group %q): %s", name, resourceGroup, err)
			}
		}

		if server.ID != nil && *server.ID != "" {
			return tf.ImportAsExistsError("azurerm_analysis_services_server", *server.ID)
		}
	}

	analysisServicesServer := analysisservices.Server{
		Name:     &name,
		Location: azure.NormalizeLocationP(d.Get("location")),
		Sku: &analysisservices.ResourceSku{
			Name: p.StringI(d.Get("sku")),
		},
		ServerProperties: expandAnalysisServicesServerProperties(d),
		Tags:             tags.ExpandI(d.Get("tags")),
	}

	future, err := client.Create(ctx, resourceGroup, name, analysisServicesServer)
	if err != nil {
		return fmt.Errorf("creating Analysis Services Server %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for completion of Analysis Services Server %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	resp, getDetailsErr := client.GetDetails(ctx, resourceGroup, name)
	if getDetailsErr != nil {
		return fmt.Errorf("retrieving Analytics Services Server %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if resp.ID == nil {
		return fmt.Errorf("cannot read ID for Analytics Services Server %q (Resource Group %q)", name, resourceGroup)
	}

	d.SetId(*resp.ID)

	return resourceArmAnalysisServicesServerRead(d, meta)
}

func resourceArmAnalysisServicesServerRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AnalysisServices.ServerClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.AnalysisServicesServerID(d.Id())
	if err != nil {
		return err
	}

	server, err := client.GetDetails(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(server.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error retrieving Analytics Services Server %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	d.Set("name", id.Name)
	d.Set("resource_group_name", id.ResourceGroup)

	if location := server.Location; location != nil {
		d.Set("location", azure.NormalizeLocation(*location))
	}

	if server.Sku != nil {
		d.Set("sku", server.Sku.Name)
	}

	if serverProps := server.ServerProperties; serverProps != nil {
		if serverProps.AsAdministrators == nil {
			d.Set("admin_users", []string{})
		} else {
			d.Set("admin_users", serverProps.AsAdministrators.Members)
		}

		enablePowerBi, fwRules := flattenAnalysisServicesServerFirewallSettings(serverProps)
		d.Set("enable_power_bi_service", enablePowerBi)
		if err := d.Set("ipv4_firewall_rule", fwRules); err != nil {
			return fmt.Errorf("Error setting `ipv4_firewall_rule`: %s", err)
		}

		d.Set("querypool_connection_mode", string(serverProps.QuerypoolConnectionMode))
		d.Set("server_full_name", serverProps.ServerFullName)

		if containerUri, ok := d.GetOk("backup_blob_container_uri"); ok {
			d.Set("backup_blob_container_uri", containerUri)
		}
	}

	return tags.FlattenAndSet(d, server.Tags)
}

func resourceArmAnalysisServicesServerUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AnalysisServices.ServerClient
	ctx, cancel := timeouts.ForUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for Azure ARM Analysis Services Server creation.")

	id, err := parse.AnalysisServicesServerID(d.Id())
	if err != nil {
		return err
	}

	analysisServicesServer := analysisservices.ServerUpdateParameters{
		ServerMutableProperties: expandAnalysisServicesServerMutableProperties(d),
		Sku: &analysisservices.ResourceSku{
			Name: p.StringI(d.Get("sku")),
		},
		Tags: tags.ExpandI(d.Get("tags")),
	}

	future, err := client.Update(ctx, id.ResourceGroup, id.Name, analysisServicesServer)
	if err != nil {
		return fmt.Errorf("Error creating Analysis Services Server %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for completion of Analysis Services Server %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	return resourceArmAnalysisServicesServerRead(d, meta)
}

func resourceArmAnalysisServicesServerDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).AnalysisServices.ServerClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.AnalysisServicesServerID(d.Id())
	if err != nil {
		return err
	}

	future, err := client.Delete(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		return fmt.Errorf("Error deleting Analysis Services Server %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error waiting for deletion of Analysis Services Server %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	return nil
}

func expandAnalysisServicesServerProperties(d *schema.ResourceData) *analysisservices.ServerProperties {
	adminUsers := expandAnalysisServicesServerAdminUsers(d)

	serverProperties := analysisservices.ServerProperties{
		AsAdministrators:     adminUsers,
		IPV4FirewallSettings: expandAnalysisServicesServerFirewallSettings(d),
	}

	if querypoolConnectionMode, ok := d.GetOk("querypool_connection_mode"); ok {
		serverProperties.QuerypoolConnectionMode = analysisservices.ConnectionMode(querypoolConnectionMode.(string))
	}

	if containerUri, ok := d.GetOk("backup_blob_container_uri"); ok {
		serverProperties.BackupBlobContainerURI = utils.String(containerUri.(string))
	}

	return &serverProperties
}

func expandAnalysisServicesServerMutableProperties(d *schema.ResourceData) *analysisservices.ServerMutableProperties {
	adminUsers := expandAnalysisServicesServerAdminUsers(d)

	serverProperties := analysisservices.ServerMutableProperties{
		AsAdministrators:        adminUsers,
		IPV4FirewallSettings:    expandAnalysisServicesServerFirewallSettings(d),
		QuerypoolConnectionMode: analysisservices.ConnectionMode(d.Get("querypool_connection_mode").(string)),
	}

	if containerUri, ok := d.GetOk("backup_blob_container_uri"); ok {
		serverProperties.BackupBlobContainerURI = utils.String(containerUri.(string))
	}

	return &serverProperties
}

func expandAnalysisServicesServerAdminUsers(d *schema.ResourceData) *analysisservices.ServerAdministrators {
	adminUsers := d.Get("admin_users").(*schema.Set)
	members := make([]string, 0)

	for _, admin := range adminUsers.List() {
		if adm, ok := admin.(string); ok {
			members = append(members, adm)
		}
	}

	return &analysisservices.ServerAdministrators{Members: &members}
}

func expandAnalysisServicesServerFirewallSettings(d *schema.ResourceData) *analysisservices.IPv4FirewallSettings {
	firewallRules := d.Get("ipv4_firewall_rule").(*schema.Set).List()
	fwRules := make([]analysisservices.IPv4FirewallRule, len(firewallRules))

	for i, v := range firewallRules {
		fwRule := v.(map[string]interface{})
		fwRules[i] = analysisservices.IPv4FirewallRule{
			FirewallRuleName: p.StringI(fwRule["name"]),
			RangeStart:       p.StringI(fwRule["range_start"]),
			RangeEnd:         p.StringI(fwRule["range_end"]),
		}
	}

	return &analysisservices.IPv4FirewallSettings{
		EnablePowerBIService: p.BoolI(d.Get("enable_power_bi_service")),
		FirewallRules:        &fwRules,
	}
}

func flattenAnalysisServicesServerFirewallSettings(serverProperties *analysisservices.ServerProperties) (enablePowerBi *bool, fwRules []interface{}) {
	if serverProperties == nil || serverProperties.IPV4FirewallSettings == nil {
		return utils.Bool(false), make([]interface{}, 0)
	}

	firewallSettings := serverProperties.IPV4FirewallSettings

	enablePowerBi = utils.Bool(false)
	if firewallSettings.EnablePowerBIService != nil {
		enablePowerBi = firewallSettings.EnablePowerBIService
	}

	fwRules = make([]interface{}, 0)
	if firewallSettings.FirewallRules != nil {
		for _, fwRule := range *firewallSettings.FirewallRules {
			output := make(map[string]interface{})

			output["name"] = p.StrOrEmpty(fwRule.FirewallRuleName)
			output["range_start"] = p.StrOrEmpty(fwRule.RangeStart)
			output["range_end"] = p.StrOrEmpty(fwRule.RangeEnd)

			fwRules = append(fwRules, output)
		}
	}

	return enablePowerBi, fwRules
}
