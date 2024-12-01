package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

// SetCacheSize is the administrative command that sets the server's cache size for
// storing previously-compiled service handlers. If you specify a smaller number
// that the current cache size, the next attempt to load a new service into the cache
// will result in discarding the oldest cache entries until the cache is the correct
// size. You must be an admin user with a valid token to perform this command.
func SetCacheSize(c *cli.Context) error {
	if c.ParameterCount() == 0 {
		return errors.ErrCacheSizeNotSpecified
	}

	size, err := strconv.Atoi(c.Parameter(0))
	if err != nil {
		return errors.ErrInvalidInteger.Context(c.Parameter(0))
	}

	cacheStatus := defs.CacheResponse{
		ServiceCountLimit: size,
	}

	err = rest.Exchange(defs.AdminCachesPath, http.MethodPost, &cacheStatus, &cacheStatus, defs.AdminAgent)
	if err != nil {
		return errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		ui.Say("msg.server.cache.updated")
	} else {
		_ = commandOutput(cacheStatus)
	}

	return nil
}

// FlushCaches is the administrative command that directs the server to
// discard any cached compilation units for service code. Subsequent service
// requests require that the service code be reloaded from disk. This is often
// used when making changes to a service, to quickly force the server to pick up
// the changes. You must be an admin user with a valid token to perform this command.
func FlushCaches(c *cli.Context) error {
	cacheStatus := defs.CacheResponse{}

	err := rest.Exchange(defs.AdminCachesPath, http.MethodDelete, nil, &cacheStatus, defs.AdminAgent)
	if err != nil {
		return err
	}

	switch ui.OutputFormat {
	case ui.JSONIndentedFormat:
		b, _ := json.MarshalIndent(cacheStatus, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		fmt.Println(string(b))

	case ui.JSONFormat:
		b, _ := json.Marshal(cacheStatus)

		fmt.Println(string(b))

	case ui.TextFormat:
		ui.Say("msg.server.cache.emptied")
	}

	return nil
}

// ShowCaches is the administrative command that displays the information about
// the server's cache of previously-compiled service programs. The current and maximum
// size of the cache, and the endpoints that are cached are listed. You must be an
// admin user with a valid token to perform this command.
func ShowCaches(c *cli.Context) error {
	var (
		found        bool
		order        string
		cacheStatus  = defs.CacheResponse{}
		url          = defs.AdminCachesPath
		t            *tables.Table
		showServices bool
		showAssets   bool
		showClass    bool
	)

	if !c.WasFound("services") && !c.WasFound("assets") {
		showServices = true
		showAssets = true
		showClass = true
	} else {
		showServices = c.Boolean("services")
		showAssets = c.Boolean("assets")
		showClass = showServices && showAssets
	}

	if order, found = c.String("order-by"); found {
		url += "?order-by=" + order
	}

	err := rest.Exchange(url, http.MethodGet, nil, &cacheStatus, defs.AdminAgent)
	if err != nil {
		return err
	}

	if ui.OutputFormat == ui.TextFormat {
		cacheAsText(cacheStatus, showServices, showAssets, showClass, t)
	} else {
		// If we aren't showing everything, only show the requested items. Loop over the items
		// array, and delete items that are not selected by the showAssets and showServices flags.
		cacheAsJSON(showClass, cacheStatus, showAssets, showServices)
	}

	return nil
}

func cacheAsJSON(showClass bool, cacheStatus defs.CacheResponse, showAssets bool, showServices bool) {
	if !showClass {
		for i := len(cacheStatus.Items) - 1; i >= 0; i-- {
			if showAssets && cacheStatus.Items[i].Class == defs.AssetCacheClass {
				continue
			} else if showServices && cacheStatus.Items[i].Class == defs.ServiceCacheClass {
				continue
			} else {
				cacheStatus.Items = append(cacheStatus.Items[:i], cacheStatus.Items[i+1:]...)
			}
		}
	}

	_ = commandOutput(cacheStatus)
}

func cacheAsText(cacheStatus defs.CacheResponse, showServices bool, showAssets bool, showClass bool, t *tables.Table) {
	fmt.Println(i18n.M("server.cache", map[string]interface{}{
		"host": cacheStatus.Hostname,
		"id":   cacheStatus.ID,
	}))

	if cacheStatus.ServiceCount > 0 && showServices || cacheStatus.AssetCount > 0 && showAssets {
		fmt.Printf("\n")

		if showClass {
			t, _ = tables.New([]string{"URL Path", "Class", "Count", "Last Used"})
			_ = t.SetAlignment(2, tables.AlignmentRight)
		} else {
			t, _ = tables.New([]string{"URL Path", "Count", "Last Used"})
			_ = t.SetAlignment(1, tables.AlignmentRight)
		}

		for _, v := range cacheStatus.Items {
			if showClass {
				_ = t.AddRowItems(v.Name, v.Class, v.Count, v.LastUsed)
			} else {
				if showAssets && v.Class == defs.AssetCacheClass {
					_ = t.AddRowItems(v.Name, v.Count, v.LastUsed)
				}

				if showServices && v.Class == defs.ServiceCacheClass {
					_ = t.AddRowItems(v.Name, v.Count, v.LastUsed)
				}
			}
		}

		_ = t.SetIndent(2)
		t.SetPagination(0, 0)

		t.Print(ui.TextFormat)
		fmt.Printf("\n")
	}

	if showAssets {
		switch cacheStatus.AssetCount {
		case 0:
			fmt.Printf("  %s\n", i18n.M("server.cache.no.assets"))

		case 1:
			fmt.Printf("  %s\n", i18n.M("server.cache.one.asset", map[string]interface{}{
				"size": cacheStatus.AssetSize,
			}))

		default:
			fmt.Printf("  %s\n", i18n.M("server.cache.assets", map[string]interface{}{
				"count": cacheStatus.AssetCount,
				"size":  cacheStatus.AssetSize,
			}))
		}
	}

	if showServices {
		switch cacheStatus.ServiceCount {
		case 0:
			fmt.Printf("  %s\n", i18n.M("server.cache.no.services", map[string]interface{}{
				"limit": cacheStatus.ServiceCountLimit,
			}))

		case 1:
			fmt.Printf("  %s\n", i18n.M("server.cache.one.service", map[string]interface{}{
				"limit": cacheStatus.ServiceCountLimit,
			}))

		default:
			fmt.Printf("  %s\n", i18n.M("server.cache.services", map[string]interface{}{
				"count": cacheStatus.ServiceCount - cacheStatus.AssetCount,
				"limit": cacheStatus.ServiceCountLimit,
			}))
		}
	}
}
