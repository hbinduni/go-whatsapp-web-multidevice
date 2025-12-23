export default {
    name: 'ChatStorageBackup',
    data() {
        return {
            loading: false,
            importFile: null,
            importResult: null,
            analyzeFile: null,
            analyzeResult: null
        }
    },
    methods: {
        openModal() {
            $('#modalChatStorageBackup').modal('show');
            this.importResult = null;
            this.importFile = null;
            this.analyzeResult = null;
            this.analyzeFile = null;
        },
        closeModal() {
            $('#modalChatStorageBackup').modal('hide');
        },
        async exportStorage() {
            this.loading = true;
            try {
                const response = await window.http.get('/chat/export', {
                    responseType: 'blob'
                });

                // Create download link
                const blob = new Blob([response.data], { type: 'application/octet-stream' });
                const url = window.URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;

                // Get filename from Content-Disposition header or use default
                const contentDisposition = response.headers['content-disposition'];
                let filename = 'chatstorage.db';
                if (contentDisposition) {
                    const match = contentDisposition.match(/filename="(.+)"/);
                    if (match) filename = match[1];
                }

                link.download = filename;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                window.URL.revokeObjectURL(url);

                showSuccessInfo('Chat storage exported successfully');
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to export chat storage');
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
        handleAnalyzeFileSelect(event) {
            const files = event.target.files;
            if (files && files.length > 0) {
                this.analyzeFile = files[0];
                this.analyzeResult = null;
            }
        },
        async importStorage() {
            if (!this.importFile) {
                showErrorInfo('Please select a backup file to import');
                return;
            }

            this.loading = true;
            this.importResult = null;

            try {
                const formData = new FormData();
                formData.append('file', this.importFile);

                const response = await window.http.post('/chat/import', formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                });

                this.importResult = response.data.results;
                showSuccessInfo('Chat storage imported successfully');
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to import chat storage');
            } finally {
                this.loading = false;
            }
        },
        async analyzeStorage() {
            if (!this.analyzeFile) {
                showErrorInfo('Please select a backup file to analyze');
                return;
            }

            this.loading = true;
            this.analyzeResult = null;

            try {
                const formData = new FormData();
                formData.append('file', this.analyzeFile);

                const response = await window.http.post('/chat/analyze', formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                });

                this.analyzeResult = response.data.results;
                showSuccessInfo('Analysis completed successfully');
            } catch (error) {
                showErrorInfo(error.response?.data?.message || 'Failed to analyze storage');
            } finally {
                this.loading = false;
            }
        },
        resetImport() {
            this.importFile = null;
            this.importResult = null;
            const fileInput = document.getElementById('importFileInput');
            if (fileInput) fileInput.value = '';
        },
        resetAnalyze() {
            this.analyzeFile = null;
            this.analyzeResult = null;
            const fileInput = document.getElementById('analyzeFileInput');
            if (fileInput) fileInput.value = '';
        }
    },
    template: `
    <div class="teal card" @click="openModal()" style="cursor: pointer">
        <div class="content">
            <a class="ui teal right ribbon label">Backup</a>
            <div class="header">Chat Storage Backup</div>
            <div class="description">
                Export, import, or analyze chat history
            </div>
        </div>
    </div>

    <!--  Modal ChatStorageBackup  -->
    <div class="ui large modal" id="modalChatStorageBackup">
        <i class="close icon"></i>
        <div class="header">
            <i class="database icon"></i>
            Chat Storage Backup
        </div>
        <div class="content">
            <div class="ui three column stackable grid">
                <!-- Export Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="download icon"></i>
                            Export Backup
                        </h3>
                        <p>Download your chat history as a SQLite database file.</p>
                        <button class="ui primary button"
                                @click="exportStorage"
                                :class="{ loading: loading }">
                            <i class="download icon"></i>
                            Export
                        </button>
                    </div>
                </div>

                <!-- Import Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="upload icon"></i>
                            Import Backup
                        </h3>
                        <p>Restore chat history from a backup file (merge mode).</p>

                        <div class="ui form">
                            <div class="field">
                                <input type="file"
                                       id="importFileInput"
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
                                <button class="ui green button"
                                        @click="importStorage"
                                        :class="{ loading: loading, disabled: !importFile }">
                                    <i class="upload icon"></i>
                                    Import
                                </button>
                                <button v-if="importFile || importResult"
                                        class="ui button"
                                        @click="resetImport">
                                    <i class="redo icon"></i>
                                </button>
                            </div>
                        </div>

                        <!-- Import Result -->
                        <div v-if="importResult" class="ui success message" style="margin-top: 1em;">
                            <div class="header">Import Complete</div>
                            <div class="ui small list">
                                <div class="item">
                                    <i class="green check icon"></i>
                                    Imported: {{ importResult.imported?.chats || 0 }} chats, {{ importResult.imported?.messages || 0 }} msgs
                                </div>
                                <div class="item">
                                    <i class="yellow minus icon"></i>
                                    Skipped: {{ importResult.skipped?.chats || 0 }} chats, {{ importResult.skipped?.messages || 0 }} msgs
                                </div>
                                <div class="item">
                                    <i class="clock icon"></i>
                                    {{ importResult.duration }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Analyze Section -->
                <div class="column">
                    <div class="ui segment" style="height: 100%;">
                        <h3 class="ui header">
                            <i class="search icon"></i>
                            Analyze Backup
                        </h3>
                        <p>View statistics and structure of a backup file.</p>

                        <div class="ui form">
                            <div class="field">
                                <input type="file"
                                       id="analyzeFileInput"
                                       accept=".db,.sqlite,.sqlite3"
                                       @change="handleAnalyzeFileSelect">
                            </div>
                            <div v-if="analyzeFile" class="field">
                                <div class="ui small info message">
                                    <i class="file icon"></i>
                                    {{ analyzeFile.name }}
                                </div>
                            </div>
                            <div class="ui buttons">
                                <button class="ui orange button"
                                        @click="analyzeStorage"
                                        :class="{ loading: loading, disabled: !analyzeFile }">
                                    <i class="search icon"></i>
                                    Analyze
                                </button>
                                <button v-if="analyzeFile || analyzeResult"
                                        class="ui button"
                                        @click="resetAnalyze">
                                    <i class="redo icon"></i>
                                </button>
                            </div>
                        </div>

                        <!-- Analyze Result -->
                        <div v-if="analyzeResult" class="ui info message" style="margin-top: 1em;">
                            <div class="header">{{ analyzeResult.filename }}</div>
                            <p><strong>Size:</strong> {{ analyzeResult.size }}</p>

                            <table class="ui very basic compact small table">
                                <thead>
                                    <tr>
                                        <th>Table</th>
                                        <th>Rows</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="table in analyzeResult.tables" :key="table.name">
                                        <td>{{ table.name }}</td>
                                        <td>{{ table.row_count }}</td>
                                    </tr>
                                </tbody>
                            </table>

                            <div class="ui divider"></div>
                            <strong>Schema:</strong>
                            <div class="ui small list">
                                <div class="item" v-for="schema in analyzeResult.schema" :key="schema.table">
                                    <i class="table icon"></i>
                                    <div class="content">
                                        <div class="header">{{ schema.table }}</div>
                                        <div class="description">{{ schema.description }}</div>
                                    </div>
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
                    <li>Export creates a copy of your local chat storage database.</li>
                    <li>Import uses merge mode - existing data is preserved, only new data is added.</li>
                    <li>Media files are not included in the backup - only chat metadata and message text.</li>
                </ul>
            </div>
        </div>
        <div class="actions">
            <div class="ui approve button">Close</div>
        </div>
    </div>
    `
}
