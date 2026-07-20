package main

import (
	"encoding/json"
	"strings"
	_ "embed"
)

//go:embed static/admin.html
var adminHTML string

func buildAdminHTML() string {
	adminKeys := []string{
		"admin_page_title",
		"admin_login_title",
		"admin_login_subtitle",
		"admin_login_username_label",
		"admin_login_password_label",
		"admin_login_btn",
		"admin_login_error_empty",
		"admin_login_fail",
		"admin_sidebar_name",
		"admin_sidebar_dashboard",
		"admin_sidebar_users",
		"admin_logout_btn",
		"admin_header_user_prefix",
		"admin_sidebar_current_user_prefix",
		"admin_page_dashboard",
		"admin_page_users",
		"admin_dashboard_total_users",
		"admin_dashboard_active_users",
		"admin_dashboard_banned_users",
		"admin_dashboard_total_requests",
		"admin_dashboard_version_title",
		"admin_dashboard_version",
		"admin_dashboard_platform",
		"admin_dashboard_requests",
		"admin_dashboard_last_request",
		"admin_dashboard_no_data",
		"admin_users_search_placeholder",
		"admin_users_new_btn",
		"admin_users_user",
		"admin_users_status",
		"admin_users_actions",
		"admin_users_normal",
		"admin_users_banned",
		"admin_users_empty",
		"admin_action_detail",
		"admin_action_unban",
		"admin_action_ban",
		"admin_action_rename",
		"admin_action_uid",
		"admin_action_delete",
		"admin_modal_create_user_title",
		"admin_modal_username_label",
		"admin_modal_password_label",
		"admin_modal_cancel",
		"admin_modal_create",
		"admin_modal_close",
		"admin_modal_ban_title",
		"admin_modal_ban_reason_label",
		"admin_modal_ban_reason_placeholder",
		"admin_modal_ban_mode_label",
		"admin_modal_ban_permanent",
		"admin_modal_ban_timed",
		"admin_modal_ban_expiry_label",
		"admin_modal_ban_confirm",
		"admin_modal_confirm_title",
		"admin_modal_confirm_ok",
		"admin_modal_alert_title",
		"admin_modal_alert_ok",
		"admin_modal_rename_title",
		"admin_modal_rename_new_label",
		"admin_modal_rename_confirm",
		"admin_modal_uid_title",
		"admin_modal_uid_new_label",
		"admin_modal_uid_confirm",
		"admin_modal_uid_current",
		"admin_modal_user_detail_title",
		"admin_modal_change_password_title",
		"admin_modal_new_password_placeholder",
		"admin_modal_change_password_btn",
		"admin_modal_confirm_action_title",
		"admin_confirm_delete_user",
		"admin_confirm_irreversible",
		"admin_confirm_unban",
		"admin_error",
		"admin_success",
		"admin_password_changed",
		"admin_create_fail",
		"admin_ban_fail",
		"admin_unban_fail",
		"admin_rename_fail",
		"admin_uid_fail",
		"admin_delete_fail",
		"admin_username_required",
		"admin_password_required",
		"admin_uid_required",
		"admin_ban_expiry_required",
		"admin_username_placeholder",
		"admin_password_placeholder",
		"admin_action_modify",
		"admin_sidebar_settings",
		"admin_settings_admin",
		"admin_settings_username",
		"admin_settings_password",
		"admin_settings_route",
		"admin_settings_old_password",
		"admin_settings_save",
		"admin_settings_saved",
		"admin_settings_old_password_required",
		"admin_settings_nothing",
	}
	m := make(map[string]string, len(adminKeys))
	for _, k := range adminKeys {
		m[k] = L(k)
	}
	langJSON, err := json.Marshal(m)
	if err != nil {
		return strings.Replace(adminHTML, "/*LANG_DATA*/", "{}", 1)
	}
	return strings.Replace(adminHTML, "/*LANG_DATA*/", string(langJSON), 1)
}
