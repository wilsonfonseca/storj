// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-container" @click="onHeaderItemClick(ProjectMemberOrderBy.NAME)">
            <p>Name</p>
            <VerticalArrows
                :isActive="getSortBy === ProjectMemberOrderBy.NAME"
                :direction="getSortDirection"/>
        </div>
        <div class="sort-header-container__added-container" @click="onHeaderItemClick(ProjectMemberOrderBy.CREATED_AT)">
            <p>Added</p>
            <VerticalArrows
                :isActive="getSortBy === ProjectMemberOrderBy.CREATED_AT"
                :direction="getSortDirection"/>
        </div>
        <div class="sort-header-container__email-container" @click="onHeaderItemClick(ProjectMemberOrderBy.EMAIL)">
            <p>Email</p>
            <VerticalArrows
                :isActive="getSortBy === ProjectMemberOrderBy.EMAIL"
                :direction="getSortDirection"/>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import { OnHeaderClickCallback, ProjectMemberOrderBy } from '@/types/projectMembers';
    import { SortDirection } from '@/types/common';
    import VerticalArrows from '@/components/common/VerticalArrows.vue';

    @Component({
        components: {
            VerticalArrows,
        },
    })
    export default class SortingListHeader extends Vue {
        @Prop({default: () => { return new Promise(() => false); }})
        private readonly onHeaderClickCallback: OnHeaderClickCallback;

        public ProjectMemberOrderBy = ProjectMemberOrderBy;

        public sortBy: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME;
        public sortDirection: SortDirection = SortDirection.ASCENDING;

        public get getSortDirection() {
            if (this.sortDirection === SortDirection.DESCENDING) {
                return SortDirection.ASCENDING;
            }

            return SortDirection.DESCENDING;
        }

        public get getSortBy() {
            return this.sortBy;
        }

        public async onHeaderItemClick(sortBy: ProjectMemberOrderBy): Promise<void> {
            if (this.sortBy != sortBy) {
                this.sortBy = sortBy;
                this.sortDirection = SortDirection.ASCENDING;

                await this.onHeaderClickCallback(this.sortBy, this.sortDirection);

                return;
            }

            if (this.sortDirection === SortDirection.DESCENDING) {
                this.sortDirection = SortDirection.ASCENDING;
            } else {
                this.sortDirection = SortDirection.DESCENDING;
            }

            await this.onHeaderClickCallback(this.sortBy, this.sortDirection);
        }
    }
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        flex-direction: row;
        height: 36px;
        background-color: rgba(255, 255, 255, 0.3);
        margin-top: 29px;

        p {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #AFB7C1;
            margin: 0;
        }

        &__name-container {
            display: flex;
            width: calc(50% - 30px);
            cursor: pointer;
            text-align: left;
            margin-left: 30px;
            align-items: center;
            justify-content: flex-start;
        }

        &__added-container {
             width: 25%;
             cursor: pointer;
             text-align: left;
             margin-left: 30px;
            display: flex;
            align-items: center;
            justify-content: flex-start;
        }

        &__email-container {
            width: 25%;
             cursor: pointer;
             text-align: left;
             display: flex;
             align-items: center;
             justify-content: flex-start;
        }
    }
</style>
