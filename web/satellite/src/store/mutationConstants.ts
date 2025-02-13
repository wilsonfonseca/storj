// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const API_KEYS_MUTATIONS = {
    FETCH: 'setAPIKeys',
    ADD: 'addAPIKey',
    DELETE: 'deleteAPIKey',
    TOGGLE_SELECTION: 'toggleSelection',
    CLEAR_SELECTION: 'clearSelection',
    CLEAR: 'clear',
};

export const NOTIFICATION_MUTATIONS = {
    ADD: 'ADD_NOTIFICATION',
    DELETE: 'DELETE_NOTIFICATION',
    PAUSE: 'PAUSE_NOTIFICATION',
    RESUME: 'RESUME_NOTIFICATION',
    CLEAR: 'CLEAR_NOTIFICATIONS',
};

export const APP_STATE_MUTATIONS = {
    TOGGLE_ADD_TEAMMEMBER_POPUP: 'TOGGLE_ADD_TEAMMEMBER_POPUP',
    TOGGLE_NEW_PROJECT_POPUP: 'TOGGLE_NEW_PROJECT_POPUP',
    TOGGLE_PROJECT_DROPDOWN: 'TOGGLE_PROJECT_DROPDOWN',
    TOGGLE_ACCOUNT_DROPDOWN: 'TOGGLE_ACCOUNT_DROPDOWN',
    TOGGLE_DELETE_PROJECT_DROPDOWN: 'TOGGLE_DELETE_PROJECT_DROPDOWN',
    TOGGLE_DELETE_ACCOUNT_DROPDOWN: 'TOGGLE_DELETE_ACCOUNT_DROPDOWN',
    TOGGLE_SORT_PM_BY_DROPDOWN: 'TOGGLE_SORT_PM_BY_DROPDOWN',
    TOGGLE_SUCCESSFUL_REGISTRATION_POPUP: 'TOGGLE_SUCCESSFUL_REGISTRATION_POPUP',
    TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP: 'TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP',
    TOGGLE_EDIT_PROFILE_POPUP: 'TOGGLE_EDIT_PROFILE_POPUP',
    TOGGLE_CHANGE_PASSWORD_POPUP: 'TOGGLE_CHANGE_PASSWORD_POPUP',
    SHOW_DELETE_PAYMENT_METHOD_POPUP: 'SHOW_DELETE_PAYMENT_METHOD_POPUP',
    SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP: 'SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP',
    CLOSE_ALL: 'CLOSE_ALL',
    CHANGE_STATE: 'CHANGE_STATE',
};

export const PROJECT_PAYMENT_METHODS_MUTATIONS = {
    FETCH: 'FETCH',
    CLEAR: 'CLEAR',
};
