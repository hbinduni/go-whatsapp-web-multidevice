export default {
    name: 'WhatsAppStoreBackup',
    data() {
        return {
            loading: false,
            stats: null,
            importFile: null,
            importResult: null,
            vacuumResult: null
        }
    },
    methods: {
        openModal() {
            $('#modalWhatsAppStoreBackup').modal('show');
            this.importResult = null;
            this.importFile = null;
            this.vacuumResult = null;
            this.loadStats();
        },
        closeModal() {
            $('#modalWhatsAppStoreBackup').modal('hide');
        },
        async loadStats() {
            this.loading = true;
            try {
                const response = await window.http.get('/admin/whatsapp-store/stats');
                this.stats = response.data.results;
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to load WhatsApp store stats');
            } finally {
                this.loading = false;
            }
        },
        async exportStore() {
            this.loading = true;
            try {
                const response = await window.http.get('/admin/whatsapp-store/export', {
                    responseType: 'blob'
                });

                // Create download link
                const blob = new Blob([response.data], { type: 'application/octet-stream' });
                const url = window.URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;

                // Get filename from Content-Disposition header or use default
                const contentDisposition = response.headers['content-disposition'];
                let filename = 'whatsapp-store.db';
                if (contentDisposition) {
                    const match = contentDisposition.match(/filename="(.+)"/);
                    if (match) filename = match[1];
                }

                link.download = filename;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                window.URL.revokeObjectURL(url);

                // Check for warning header
                const warning = response.headers['x-export-warning'];
                if (warning) {
                    showSuccessInfo('WhatsApp store exported. Warning: ' + warning);
                } else {
                    showSuccessInfo('WhatsApp store exported successfully');
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to export WhatsApp store');
            } finally {
                this.loading = false;
            }
        },
        handleFileSelect(event) {
            const files = event.target.files;
            if (files && files.length > 0) {
                this.importFile = files[0];
            }
        },
        async importStore() {
            if (!this.importFile) {
                showErrorInfo('Please select a backup file to import');
                return;
            }

            // Confirm import - this is a critical operation
            if (!confirm('WARNING: Importing will replace your current WhatsApp credentials and keys. You will need to restart the application after import. Are you sure you want to continue?')) {
                return;
            }

            this.loading = true;
            this.importResult = null;

            try {
                const formData = new FormData();
                formData.append('file', this.importFile);

                const response = await window.http.post('/admin/whatsapp-store/import', formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                });

                this.importResult = response.data.results;
                if (this.importResult.requires_restart) {
                    showSuccessInfo('Import successful! Please restart the application to apply changes.');
                } else {
                    showSuccessInfo('WhatsApp store imported successfully');
                }
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to import WhatsApp store');
            } finally {
                this.loading = false;
            }
        },
        async runVacuum() {
            this.loading = true;
            this.vacuumResult = null;
            try {
                const response = await window.http.post('/admin/whatsapp-store/vacuum');
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
        resetImport() {
            this.importFile = null;
            this.importResult = null;
            const fileInput = document.getElementById('waStoreImportFileInput');
            if (fileInput) fileInput.value = '';
        },
        formatSize(bytes) {
            if (!bytes) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }
    },
    template: `
    <div class="orange card" @click="openModal()" style="cursor: pointer">
        <div class="content">
            <a class="ui orange right ribbon label">Keys</a>
            <div class="header">WhatsApp Store Backup</div>
            <div class="description">
                Export, import, or optimize device credentials
            </div>
        </div>
    </div>

    <!--  Modal WhatsAppStoreBackup  -->
    <div class="ui large modal" id="modalWhatsAppStoreBackup">
        <i class="close icon"></i>
        <div class="header">
            <i class="key icon"></i>
            WhatsApp Store Backup
        </div>
        <div class="content">
            <!-- Stats Section -->
            <div class="ui segment">
                <h3 class="ui header">
                    <i class="chart bar icon"></i>
                    Store Statistics
                    <button class="ui mini right floated button" @click="loadStats" :class="{ loading: loading }">
                        <i class="sync icon"></i> Refresh
                    </button>
                </h3>

                <div v-if="stats" class="ui four column stackable grid">
                    <div class="column">
                        <div class="ui small statistic">
                            <div class="value">{{ stats.database_size }}</div>
                            <div class="label">Main DB Size</div>
                        </div>
                    </div>
                    <div class="column">
                        <div class="ui small statistic">
                            <div class="value">{{ stats.device_count }}</div>
                            <div class="label">Devices</div>
                        </div>
                    </div>
                    <div class="column">
                        <div class="ui small statistic">
                            <div class="value">
                                <i :class="[stats.is_connected ? 'green' : 'red', 'circle icon']"></i>
                            </div>
                            <div class="label">{{ stats.is_connected ? 'Connected' : 'Disconnected' }}</div>
                        </div>
                    </div>
                    <div class="column">
                        <div class="ui small statistic">
                            <div class="value">
                                <i :class="[stats.is_logged_in ? 'green' : 'grey', 'user icon']"></i>
                            </div>
                            <div class="label">{{ stats.is_logged_in ? 'Logged In' : 'Not Logged In' }}</div>
                        </div>
                    </div>
                </div>

                <div v-if="stats && stats.device_jid" class="ui small message" style="margin-top: 1em;">
                    <i class="mobile icon"></i>
                    Device JID: <strong>{{ stats.device_jid }}</strong>
                </div>

                <div v-if="stats && stats.has_keys_db" class="ui small info message" style="margin-top: 0.5em;">
                    <i class="key icon"></i>
                    Keys DB Size: <strong>{{ stats.keys_db_size }}</strong>
                </div>
            </div>

            <div class="ui three column stackable grid">
                <!-- Export Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="download icon"></i>
                            Export Backup
                        </h3>
                        <p>Download your WhatsApp device credentials and keys.</p>
                        <button class="ui primary button"
                                @click="exportStore"
                                :class="{ loading: loading }">
                            <i class="download icon"></i>
                            Export
                        </button>
                        <div class="ui tiny message" style="margin-top: 1em;">
                            <i class="info circle icon"></i>
                            Contains encryption keys. Store securely!
                        </div>
                    </div>
                </div>

                <!-- Import Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="upload icon"></i>
                            Import Backup
                        </h3>
                        <p>Restore device credentials from a backup file.</p>

                        <div class="ui form">
                            <div class="field">
                                <input type="file"
                                       id="waStoreImportFileInput"
                                       accept=".db,.sqlite,.sqlite3"
                                       @change="handleFileSelect">
                            </div>
                            <div v-if="importFile" class="field">
                                <div class="ui small info message">
                                    <i class="file icon"></i>
                                    {{ importFile.name }}
                                </div>
                            </div>
                            <div class="ui buttons">
                                <button class="ui orange button"
                                        @click="importStore"
                                        :class="{ loading: loading, disabled: !importFile || (stats && stats.is_connected) }">
                                    <i class="upload icon"></i>
                                    Import
                                </button>
                                <button v-if="importFile || importResult"
                                        class="ui button"
                                        @click="resetImport">
                                    <i class="redo icon"></i>
                                </button>
                            </div>
                            <div v-if="stats && stats.is_connected" class="ui tiny warning message" style="margin-top: 0.5em;">
                                <i class="warning icon"></i>
                                Disconnect first to import
                            </div>
                        </div>

                        <!-- Import Result -->
                        <div v-if="importResult" class="ui success message" style="margin-top: 1em;">
                            <div class="header">{{ importResult.status }}</div>
                            <p>{{ importResult.message }}</p>
                            <div class="ui small list">
                                <div class="item" v-if="importResult.main_db_imported">
                                    <i class="green check icon"></i>
                                    Main database imported
                                </div>
                                <div class="item" v-if="importResult.keys_db_imported">
                                    <i class="green check icon"></i>
                                    Keys database imported
                                </div>
                                <div class="item" v-if="importResult.requires_restart">
                                    <i class="orange warning icon"></i>
                                    Restart required
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Vacuum Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="compress icon"></i>
                            Optimize Database
                        </h3>
                        <p>Run VACUUM to reclaim space and optimize performance.</p>

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
                                    <strong>Main DB:</strong>
                                    {{ vacuumResult.main_db.size_before }} → {{ vacuumResult.main_db.size_after }}
                                    ({{ vacuumResult.main_db.reclaimed }} reclaimed)
                                </div>
                                <div class="item" v-if="vacuumResult.keys_db">
                                    <strong>Keys DB:</strong>
                                    {{ vacuumResult.keys_db.size_before }} → {{ vacuumResult.keys_db.size_after }}
                                    ({{ vacuumResult.keys_db.reclaimed }} reclaimed)
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="ui red message">
                <div class="header">
                    <i class="exclamation triangle icon"></i>
                    Critical Information
                </div>
                <ul class="list">
                    <li><strong>This database contains your WhatsApp encryption keys and device credentials.</strong></li>
                    <li>Export creates a backup of your login session - store it securely!</li>
                    <li>Import replaces your current credentials - <strong>you must disconnect first</strong>.</li>
                    <li>After import, <strong>restart the application</strong> to apply changes.</li>
                    <li>If you import an old backup, you may need to re-link your device.</li>
                </ul>
            </div>
        </div>
        <div class="actions">
            <div class="ui approve button">Close</div>
        </div>
    </div>
    `
}
