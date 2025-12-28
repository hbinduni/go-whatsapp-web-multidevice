export default {
    name: 'AdminStorage',
    data() {
        return {
            loading: false,
            stats: null,
            vacuumResult: null,
            cleanupPattern: '',
            cleanupResult: null,
            cleanupDryRun: true
        }
    },
    methods: {
        openModal() {
            $('#modalAdminStorage').modal('show');
            this.loadStats();
        },
        closeModal() {
            $('#modalAdminStorage').modal('hide');
        },
        async loadStats() {
            this.loading = true;
            try {
                const response = await window.http.get('/admin/storage/stats');
                this.stats = response.data.results;
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to load storage stats');
            } finally {
                this.loading = false;
            }
        },
        async runVacuum() {
            this.loading = true;
            this.vacuumResult = null;
            try {
                const response = await window.http.post('/admin/storage/vacuum');
                this.vacuumResult = response.data.results;
                showSuccessInfo('Database vacuum completed successfully');
                // Reload stats to show new size
                await this.loadStats();
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to vacuum database');
            } finally {
                this.loading = false;
            }
        },
        async runCleanup() {
            if (!this.cleanupPattern) {
                showErrorInfo('Please enter a pattern to match (e.g., %@broadcast)');
                return;
            }

            this.loading = true;
            this.cleanupResult = null;
            try {
                const response = await window.http.post('/admin/storage/cleanup', {
                    pattern: this.cleanupPattern,
                    dry_run: this.cleanupDryRun
                });
                this.cleanupResult = response.data.results;
                if (this.cleanupDryRun) {
                    showSuccessInfo('Dry run completed - no changes made');
                } else {
                    showSuccessInfo('Cleanup completed successfully');
                    // Reload stats
                    await this.loadStats();
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to run cleanup');
            } finally {
                this.loading = false;
            }
        },
        formatDate(dateStr) {
            if (!dateStr) return 'N/A';
            return moment(dateStr).format('YYYY-MM-DD HH:mm:ss');
        }
    },
    template: `
    <div class="purple card" @click="openModal()" style="cursor: pointer">
        <div class="content">
            <a class="ui purple right ribbon label">Admin</a>
            <div class="header">Storage Management</div>
            <div class="description">
                View stats, cleanup, and optimize database
            </div>
        </div>
    </div>

    <!--  Modal AdminStorage  -->
    <div class="ui large modal" id="modalAdminStorage">
        <i class="close icon"></i>
        <div class="header">
            <i class="cog icon"></i>
            Storage Management
        </div>
        <div class="content">
            <!-- Stats Section -->
            <div class="ui segment">
                <h3 class="ui header">
                    <i class="chart bar icon"></i>
                    Storage Statistics
                    <button class="ui mini right floated button" @click="loadStats" :class="{ loading: loading }">
                        <i class="sync icon"></i> Refresh
                    </button>
                </h3>

                <div v-if="stats" class="ui four column stackable grid">
                    <div class="column">
                        <div class="ui statistic">
                            <div class="value">{{ stats.total_chats }}</div>
                            <div class="label">Chats</div>
                        </div>
                    </div>
                    <div class="column">
                        <div class="ui statistic">
                            <div class="value">{{ stats.total_messages }}</div>
                            <div class="label">Messages</div>
                        </div>
                    </div>
                    <div class="column">
                        <div class="ui statistic">
                            <div class="value">{{ stats.database_size }}</div>
                            <div class="label">DB Size</div>
                        </div>
                    </div>
                    <div class="column">
                        <div class="ui statistic">
                            <div class="value">{{ stats.empty_chats }}</div>
                            <div class="label">Empty Chats</div>
                        </div>
                    </div>
                </div>

                <div v-if="stats" class="ui divider"></div>

                <div v-if="stats" class="ui small list">
                    <div class="item">
                        <i class="image icon"></i>
                        Media Messages: {{ stats.media_messages }}
                    </div>
                    <div class="item">
                        <i class="comment icon"></i>
                        Text Messages: {{ stats.text_messages }}
                    </div>
                    <div class="item" v-if="stats.oldest_message">
                        <i class="calendar minus icon"></i>
                        Oldest: {{ formatDate(stats.oldest_message) }}
                    </div>
                    <div class="item" v-if="stats.newest_message">
                        <i class="calendar plus icon"></i>
                        Newest: {{ formatDate(stats.newest_message) }}
                    </div>
                </div>

                <div v-if="!stats && !loading" class="ui placeholder segment">
                    <div class="ui icon header">
                        <i class="chart bar outline icon"></i>
                        Click Refresh to load statistics
                    </div>
                </div>
            </div>

            <div class="ui two column stackable grid">
                <!-- Vacuum Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="compress icon"></i>
                            Optimize Database
                        </h3>
                        <p>Run SQLite VACUUM to reclaim unused space and optimize performance.</p>

                        <button class="ui blue button"
                                @click="runVacuum"
                                :class="{ loading: loading }">
                            <i class="compress icon"></i>
                            Run Vacuum
                        </button>

                        <div v-if="vacuumResult" class="ui success message" style="margin-top: 1em;">
                            <div class="header">Vacuum Complete</div>
                            <div class="ui small list">
                                <div class="item">
                                    <i class="arrow down icon"></i>
                                    Before: {{ vacuumResult.size_before }}
                                </div>
                                <div class="item">
                                    <i class="arrow up icon"></i>
                                    After: {{ vacuumResult.size_after }}
                                </div>
                                <div class="item">
                                    <i class="green check icon"></i>
                                    Reclaimed: {{ vacuumResult.reclaimed }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Cleanup Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="eraser icon"></i>
                            Cleanup Chats
                        </h3>
                        <p>Delete chats matching a pattern (uses SQL LIKE).</p>

                        <div class="ui form">
                            <div class="field">
                                <label>Pattern</label>
                                <input type="text"
                                       v-model="cleanupPattern"
                                       placeholder="%@broadcast">
                                <small class="ui grey text">Examples: %@broadcast, status@%, 123%</small>
                            </div>
                            <div class="field">
                                <div class="ui checkbox">
                                    <input type="checkbox" v-model="cleanupDryRun">
                                    <label>Dry Run (preview only)</label>
                                </div>
                            </div>
                            <button class="ui orange button"
                                    @click="runCleanup"
                                    :class="{ loading: loading, disabled: !cleanupPattern }">
                                <i class="eraser icon"></i>
                                {{ cleanupDryRun ? 'Preview' : 'Delete' }}
                            </button>
                        </div>

                        <div v-if="cleanupResult"
                             :class="['ui message', cleanupResult.dry_run ? 'info' : 'success']"
                             style="margin-top: 1em;">
                            <div class="header">{{ cleanupResult.dry_run ? 'Dry Run Result' : 'Cleanup Complete' }}</div>
                            <div class="ui small list">
                                <div class="item">
                                    <i class="comments icon"></i>
                                    Chats {{ cleanupResult.dry_run ? 'to delete' : 'deleted' }}: {{ cleanupResult.chats_deleted }}
                                </div>
                                <div class="item">
                                    <i class="comment icon"></i>
                                    Messages {{ cleanupResult.dry_run ? 'to delete' : 'deleted' }}: {{ cleanupResult.messages_deleted }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="ui warning message">
                <div class="header">
                    <i class="exclamation triangle icon"></i>
                    Important Notes
                </div>
                <ul class="list">
                    <li><strong>Vacuum:</strong> Optimizes database but may take a few seconds for large databases.</li>
                    <li><strong>Cleanup:</strong> Permanently deletes data. Use dry run first to preview changes.</li>
                    <li><strong>Pattern:</strong> Use % as wildcard. Example: %@broadcast matches status@broadcast.</li>
                </ul>
            </div>
        </div>
        <div class="actions">
            <div class="ui approve button">Close</div>
        </div>
    </div>
    `
}
