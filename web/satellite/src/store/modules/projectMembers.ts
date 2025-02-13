// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    ProjectMember,
    ProjectMemberCursor,
    ProjectMemberOrderBy,
    ProjectMembersApi,
    ProjectMembersPage,
} from '@/types/projectMembers';
import { SortDirection } from '@/types/common';
import { StoreModule } from '@/store';

export const PROJECT_MEMBER_MUTATIONS = {
    FETCH: 'fetchProjectMembers',
    TOGGLE_SELECTION: 'toggleSelection',
    CLEAR_SELECTION: 'clearSelection',
    CLEAR: 'clearProjectMembers',
    CHANGE_SORT_ORDER: 'changeProjectMembersSortOrder',
    CHANGE_SORT_ORDER_DIRECTION: 'changeProjectMembersSortOrderDirection',
    SET_SEARCH_QUERY: 'setProjectMembersSearchQuery',
    SET_PAGE: 'setProjectMembersPage',
};

const {
    FETCH,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
    CLEAR,
    CHANGE_SORT_ORDER,
    CHANGE_SORT_ORDER_DIRECTION,
    SET_SEARCH_QUERY,
    SET_PAGE,
} = PROJECT_MEMBER_MUTATIONS;

class ProjectMembersState {
    public cursor: ProjectMemberCursor = new ProjectMemberCursor();
    public page: ProjectMembersPage = new ProjectMembersPage();
}

export function makeProjectMembersModule(api: ProjectMembersApi): StoreModule<ProjectMembersState> {
    return {
        state: new ProjectMembersState(),
        mutations: {
            [FETCH](state: ProjectMembersState, page: ProjectMembersPage) {
                state.page = page;
            },
            [SET_PAGE](state: ProjectMembersState, page: number) {
                state.cursor.page = page;
            },
            [SET_SEARCH_QUERY](state: ProjectMembersState, search: string) {
                state.cursor.search = search;
            },
            [CHANGE_SORT_ORDER](state: ProjectMembersState, order: ProjectMemberOrderBy) {
                state.cursor.order = order;
            },
            [CHANGE_SORT_ORDER_DIRECTION](state: ProjectMembersState, direction: SortDirection) {
                state.cursor.orderDirection = direction;
            },
            [CLEAR](state: ProjectMembersState) {
                state.cursor = new ProjectMemberCursor();
                state.page = new ProjectMembersPage();
            },
            [TOGGLE_SELECTION](state: ProjectMembersState, projectMemberId: string) {
                state.page.projectMembers = state.page.projectMembers.map((projectMember: ProjectMember) => {
                    if (projectMember.user.id === projectMemberId) {
                        projectMember.isSelected = !projectMember.isSelected;
                    }

                    return projectMember;
                });
            },
            [CLEAR_SELECTION](state: ProjectMembersState) {
                state.page.projectMembers = state.page.projectMembers.map((projectMember: ProjectMember) => {
                    projectMember.isSelected = false;

                    return projectMember;
                });
            },
        },
        actions: {
            addProjectMembers: async function ({rootGetters}: any, emails: string[]): Promise<void> {
                const projectId = rootGetters.selectedProject.id;

                await api.add(projectId, emails);
            },
            deleteProjectMembers: async function ({rootGetters}: any, projectMemberEmails: string[]): Promise<void> {
                const projectId = rootGetters.selectedProject.id;

                await api.delete(projectId, projectMemberEmails);
            },
            fetchProjectMembers: async function ({commit, rootGetters, state}: any, page: number): Promise<ProjectMembersPage> {
                const projectID = rootGetters.selectedProject.id;

                commit(SET_PAGE, page);

                const projectMembersPage: ProjectMembersPage = await api.get(projectID, state.cursor);

                commit(FETCH, projectMembersPage);

                return projectMembersPage;
            },
            setProjectMembersSearchQuery: function ({commit}, search: string) {
                commit(SET_SEARCH_QUERY, search);
            },
            setProjectMembersSortingBy: function ({commit}, order: ProjectMemberOrderBy) {
                commit(CHANGE_SORT_ORDER, order);
            },
            setProjectMembersSortingDirection: function ({commit}, direction: SortDirection) {
                commit(CHANGE_SORT_ORDER_DIRECTION, direction);
            },
            clearProjectMembers: function ({commit}) {
                commit(CLEAR);
            },
            toggleProjectMemberSelection: function ({commit}: any, projectMemberId: string) {
                commit(TOGGLE_SELECTION, projectMemberId);
            },
            clearProjectMemberSelection: function ({commit}: any) {
                commit(CLEAR_SELECTION);
            },
        },
        getters: {
            selectedProjectMembers: (state: any) => state.page.projectMembers.filter((member: ProjectMember) => member.isSelected),
        }
    };
}
