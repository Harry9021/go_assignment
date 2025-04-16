import React, { useState, useEffect } from 'react';
import './App.css';

function App() {
  // State for source and target selection
  const [source, setSource] = useState('clickhouse');
  const [target, setTarget] = useState('flatfile');

  // ClickHouse config state
  const [clickhouseConfig, setClickhouseConfig] = useState({
    host: 'da2nakjy9k.ap-south-1.aws.clickhouse.cloud',
    port: '9440',
    database: 'default',
    username: 'default',
    jwtToken: 'JuGG4kUe~0tZe',
    isHttps: false,
  });

  // Flat file config state
  const [flatFileConfig, setFlatFileConfig] = useState({
    fileName: '',
    delimiter: ',',
    hasHeader: true,
  });

  // State for available tables, columns, and selected items
  const [tables, setTables] = useState([]);
  const [selectedTables, setSelectedTables] = useState([]);
  const [tableName, setTableName] = useState('');
  const [availableColumns, setAvailableColumns] = useState([]);
  const [selectedColumns, setSelectedColumns] = useState([]);
  const [joinCondition, setJoinCondition] = useState('');

  // State for data preview
  const [previewData, setPreviewData] = useState([]);
  const [previewLimit, setPreviewLimit] = useState(10);

  // State for ingestion process
  const [status, setStatus] = useState('idle'); // idle, connecting, fetching, ingesting, completed, error
  const [message, setMessage] = useState('');
  const [recordCount, setRecordCount] = useState(0);
  const [isMultiTableJoin, setIsMultiTableJoin] = useState(false);

  // Target table name
  const [targetTableName, setTargetTableName] = useState('');

  // Update source or target
  const handleSourceChange = (e) => {
    setSource(e.target.value);
    // Reset selected items when source changes
    setSelectedTables([]);
    setTableName('');
    setAvailableColumns([]);
    setSelectedColumns([]);
    setPreviewData([]);
  };

  const handleTargetChange = (e) => {
    setTarget(e.target.value);
  };

  // Update ClickHouse configuration
  const handleClickhouseConfigChange = (e) => {
    const { name, value, type, checked } = e.target;
    setClickhouseConfig({
      ...clickhouseConfig,
      [name]: type === 'checkbox' ? checked : value,
    });
  };

  // Update flat file configuration
  const handleFlatFileConfigChange = (e) => {
    const { name, value, type, checked } = e.target;
    setFlatFileConfig({
      ...flatFileConfig,
      [name]: type === 'checkbox' ? checked : value,
    });
  };

  // Connect to ClickHouse and fetch tables
  const handleConnectClickHouse = async () => {
    try {
      setStatus('connecting');
      setMessage('Connecting to ClickHouse...');

      const response = await fetch('http://localhost:8080/api/clickhouse/tables', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(clickhouseConfig),
      });

      const data = await response.json();

      if (data.success) {
        setTables(data.data);
        setStatus('idle');
        setMessage(`Connected to ClickHouse. Found ${data.count} tables.`);
      } else {
        setStatus('error');
        setMessage(`Error: ${data.error}`);
      }
    } catch (error) {
      setStatus('error');
      setMessage(`Connection error: ${error.message}`);
    }
  };

  // Load columns for selected table
  const handleLoadColumns = async () => {
    if (!tableName && selectedTables.length === 0) {
      setMessage('Please select a table first');
      return;
    }

    try {
      setStatus('fetching');
      setMessage('Loading columns...');

      const tableToUse = tableName || selectedTables[0];

      const response = await fetch('http://localhost:8080/api/clickhouse/columns', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          config: clickhouseConfig,
          tableName: tableToUse,
        }),
      });

      const data = await response.json();

      if (data.success) {
        console.log(data.data)
        setAvailableColumns(data.data);
        setStatus('idle');
        setMessage(`Loaded ${data.count} columns from table ${tableToUse}.`);
      } else {
        setStatus('error');
        setMessage(`Error loading columns: ${data.error}`);
      }
    } catch (error) {
      setStatus('error');
      setMessage(`Error: ${error.message}`);
    }
  };

  // Load schema from flat file
  const handleLoadFlatFileSchema = async () => {
    if (!flatFileConfig.fileName) {
      setMessage('Please enter a file name');
      return;
    }

    try {
      setStatus('fetching');
      setMessage('Loading file schema...');

      const response = await fetch('http://localhost:8080/api/flatfile/schema', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(flatFileConfig),
      });

      const data = await response.json();

      if (data.success) {
        setAvailableColumns(data.data);
        setStatus('idle');
        setMessage(`Loaded schema from ${flatFileConfig.fileName}.`);
      } else {
        setStatus('error');
        setMessage(`Error loading schema: ${data.error}`);
      }
    } catch (error) {
      setStatus('error');
      setMessage(`Error: ${error.message}`);
    }
  };

  // Toggle a column selection
  const handleColumnToggle = (columnName) => {
    if (selectedColumns.includes(columnName)) {
      setSelectedColumns(selectedColumns.filter(col => col !== columnName));
    } else {
      setSelectedColumns([...selectedColumns, columnName]);
    }
  };

  // Toggle select all columns
  const handleSelectAllColumns = () => {
    if (selectedColumns.length === availableColumns.length) {
      setSelectedColumns([]);
    } else {
      setSelectedColumns(availableColumns.map(col => col.Name || col));
    }
  };

  // Toggle a table selection for join
  const handleTableToggle = (tableName) => {
    if (selectedTables.includes(tableName)) {
      setSelectedTables(selectedTables.filter(t => t !== tableName));
    } else {
      setSelectedTables([...selectedTables, tableName]);
    }
  };

  // Preview data before ingestion
  const handlePreviewData = async () => {
    if (source === 'clickhouse' && !tableName && selectedTables.length === 0) {
      setMessage('Please select a table first');
      return;
    }

    if (source === 'flatfile' && !flatFileConfig.fileName) {
      setMessage('Please enter a file name');
      return;
    }

    if (selectedColumns.length === 0) {
      setMessage('Please select at least one column');
      return;
    }

    try {
      setStatus('fetching');
      setMessage('Loading preview data...');

      const requestBody = {
        source,
        target,
        clickhouseConfig,
        flatFileConfig,
        tableName,
        selectedTables,
        joinCondition,
        selectedColumns,
        previewOnly: true,
        previewLimit,
      };

      const response = await fetch('http://localhost:8080/api/preview', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });

      const data = await response.json();

      if (data.success) {
        setPreviewData(data.data);
        console.log(data.data)
        setStatus('idle');
        setMessage(`Preview loaded ${data.count} records.`);
      } else {
        setStatus('error');
        setMessage(`Error loading preview: ${data.error}`);
      }
    } catch (error) {
      setStatus('error');
      setMessage(`Error: ${error.message}`);
    }
  };

  // Start the ingestion process
  const handleStartIngestion = async () => {
    if (source === 'clickhouse' && !tableName && selectedTables.length === 0) {
      setMessage('Please select a table first');
      return;
    }

    if (source === 'flatfile' && !flatFileConfig.fileName) {
      setMessage('Please enter a file name');
      return;
    }

    if (selectedColumns.length === 0) {
      setMessage('Please select at least one column');
      return;
    }

    if (target === 'clickhouse' && !targetTableName) {
      setMessage('Please enter a target table name');
      return;
    }

    if (target === 'flatfile' && !flatFileConfig.fileName) {
      setMessage('Please enter a target file name');
      return;
    }

    try {
      setStatus('ingesting');
      setMessage('Starting data ingestion...');

      const requestBody = {
        source,
        target,
        clickhouseConfig,
        flatFileConfig,
        tableName,
        targetTableName,
        selectedTables,
        joinCondition,
        selectedColumns,
        previewOnly: false,
      };

      const response = await fetch('http://localhost:8080/api/ingest', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      });

      const data = await response.json();

      if (data.success) {
        setStatus('completed');
        setRecordCount(data.count);
        setMessage(`Ingestion completed successfully. ${data.count} records processed.`);
      } else {
        setStatus('error');
        setMessage(`Ingestion error: ${data.error}`);
      }
    } catch (error) {
      setStatus('error');
      setMessage(`Error: ${error.message}`);
    }
  };

  // Toggle multi-table join mode
  const handleToggleJoinMode = () => {
    setIsMultiTableJoin(!isMultiTableJoin);
    // Reset selections when toggling mode
    setSelectedTables([]);
    setTableName('');
    setJoinCondition('');
  };

  return (
    <div className="min-h-screen bg-gray-100 py-6 flex flex-col justify-center sm:py-12">
      <div className="relative py-3 sm:max-w-5xl sm:mx-auto">
        <div className="relative px-4 py-10 bg-white shadow-lg sm:rounded-3xl sm:p-20">
          <h1 className="text-3xl font-bold text-center mb-8">
            ClickHouse & Flat File Data Ingestion Tool
          </h1>

          {/* Source and Target Selection */}
          <div className="mb-8 grid grid-cols-2 gap-4">
            <div>
              <h2 className="text-xl font-semibold mb-4">Source</h2>
              <div className="flex space-x-4">
                <label className="inline-flex items-center">
                  <input
                    type="radio"
                    className="form-radio"
                    value="clickhouse"
                    checked={source === 'clickhouse'}
                    onChange={handleSourceChange}
                  />
                  <span className="ml-2">ClickHouse</span>
                </label>
                <label className="inline-flex items-center">
                  <input
                    type="radio"
                    className="form-radio"
                    value="flatfile"
                    checked={source === 'flatfile'}
                    onChange={handleSourceChange}
                  />
                  <span className="ml-2">Flat File</span>
                </label>
              </div>
            </div>

            <div>
              <h2 className="text-xl font-semibold mb-4">Target</h2>
              <div className="flex space-x-4">
                <label className="inline-flex items-center">
                  <input
                    type="radio"
                    className="form-radio"
                    value="clickhouse"
                    checked={target === 'clickhouse'}
                    onChange={handleTargetChange}
                  />
                  <span className="ml-2">ClickHouse</span>
                </label>
                <label className="inline-flex items-center">
                  <input
                    type="radio"
                    className="form-radio"
                    value="flatfile"
                    checked={target === 'flatfile'}
                    onChange={handleTargetChange}
                  />
                  <span className="ml-2">Flat File</span>
                </label>
              </div>
            </div>
          </div>

          {/* Source Configuration */}
          <div className="mb-8">
            <h2 className="text-xl font-semibold mb-4">
              {source === 'clickhouse' ? 'ClickHouse Source Configuration' : 'Flat File Source Configuration'}
            </h2>

            {source === 'clickhouse' && (
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700">Host</label>
                  <input
                    type="text"
                    name="host"
                    value={clickhouseConfig.host}
                    onChange={handleClickhouseConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700">Port</label>
                  <input
                    type="text"
                    name="port"
                    value={clickhouseConfig.port}
                    onChange={handleClickhouseConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700">Database</label>
                  <input
                    type="text"
                    name="database"
                    value={clickhouseConfig.database}
                    onChange={handleClickhouseConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700">Username</label>
                  <input
                    type="text"
                    name="username"
                    value={clickhouseConfig.username}
                    onChange={handleClickhouseConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                <div className="col-span-2">
                  <label className="block text-sm font-medium text-gray-700">JWT Token</label>
                  <input
                    type="password"
                    name="jwtToken"
                    value={clickhouseConfig.jwtToken}
                    onChange={handleClickhouseConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                {/* <div className="col-span-2">
                  <label className="inline-flex items-center">
                    <input
                      type="checkbox"
                      name="isHttps"
                      checked={clickhouseConfig.isHttps}
                      onChange={handleClickhouseConfigChange}
                      className="rounded border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                    <span className="ml-2">Use HTTPS</span>
                  </label>
                </div> */}

                <div className="col-span-2">
                  <button
                    onClick={handleConnectClickHouse}
                    className="w-full inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                    disabled={status === 'connecting' || status === 'fetching' || status === 'ingesting'}
                  >
                    Connect to ClickHouse
                  </button>
                </div>

                {tables.length > 0 && (
                  <div className="col-span-2">
                    <div className="flex items-center justify-between mb-2">
                      <label className="block text-sm font-medium text-gray-700">
                        {isMultiTableJoin ? 'Select Tables for Join' : 'Select Table'}
                      </label>
                      <button
                        onClick={handleToggleJoinMode}
                        className="text-sm text-indigo-600 hover:text-indigo-900"
                      >
                        {isMultiTableJoin ? 'Switch to Single Table' : 'Enable Multi-Table Join'}
                      </button>
                    </div>

                    {isMultiTableJoin ? (
                      <div className="border rounded-md p-2 max-h-40 overflow-y-auto">
                        {tables.map((table) => (
                          <div key={table} className="flex items-center mb-1">
                            <input
                              type="checkbox"
                              id={`table-${table}`}
                              checked={selectedTables.includes(table)}
                              onChange={() => handleTableToggle(table)}
                              className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                            />
                            <label htmlFor={`table-${table}`} className="ml-2 block text-sm text-gray-900">
                              {table}
                            </label>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <select
                        value={tableName}
                        onChange={(e) => setTableName(e.target.value)}
                        className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                      >
                        <option value="">Select a table</option>
                        {tables.map((table) => (
                          <option key={table} value={table}>{table}</option>
                        ))}
                      </select>
                    )}
                  </div>
                )}

                {/* {isMultiTableJoin && selectedTables.length > 1 && (
                  <div className="col-span-2">
                    <label className="block text-sm font-medium text-gray-700">JOIN Condition</label>
                    <input
                      type="text"
                      value={joinCondition}
                      onChange={(e) => setJoinCondition(e.target.value)}
                      placeholder="e.g., table1.id = table2.table1_id"
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                    <p className="mt-1 text-xs text-gray-500">
                      Specify the condition to join tables (e.g., table1.id = table2.table1_id)
                    </p>
                  </div>
                )} */}

                <div className="col-span-2">
                  <button
                    onClick={handleLoadColumns}
                    className="w-full inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                    disabled={(!tableName && selectedTables.length === 0) || status === 'connecting' || status === 'fetching' || status === 'ingesting'}
                  >
                    Load Columns
                  </button>
                </div>
              </div>
            )}

            {source === 'flatfile' && (
              <div className="grid grid-cols-2 gap-4">
                <div className="col-span-2">
                  <label className="block text-sm font-medium text-gray-700">File Name/Path</label>
                  <input
                    type="text"
                    name="fileName"
                    value={flatFileConfig.fileName}
                    onChange={handleFlatFileConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700">Delimiter</label>
                  <input
                    type="text"
                    name="delimiter"
                    value={flatFileConfig.delimiter}
                    onChange={handleFlatFileConfigChange}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>

                <div>
                  <label className="inline-flex items-center mt-6">
                    <input
                      type="checkbox"
                      name="hasHeader"
                      checked={flatFileConfig.hasHeader}
                      onChange={handleFlatFileConfigChange}
                      className="rounded border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                    <span className="ml-2">Has Header Row</span>
                  </label>
                </div>

                <div className="col-span-2">
                  <button
                    onClick={handleLoadFlatFileSchema}
                    className="w-full inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                    disabled={!flatFileConfig.fileName || status === 'connecting' || status === 'fetching' || status === 'ingesting'}
                  >
                    Load File Schema
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Target Configuration */}
          {source !== target && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold mb-4">
                {target === 'clickhouse' ? 'ClickHouse Target Configuration' : 'Flat File Target Configuration'}
              </h2>

              {target === 'clickhouse' && (
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Host</label>
                    <input
                      type="text"
                      name="host"
                      value={clickhouseConfig.host}
                      onChange={handleClickhouseConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700">Port</label>
                    <input
                      type="text"
                      name="port"
                      value={clickhouseConfig.port}
                      onChange={handleClickhouseConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700">Database</label>
                    <input
                      type="text"
                      name="database"
                      value={clickhouseConfig.database}
                      onChange={handleClickhouseConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700">Username</label>
                    <input
                      type="text"
                      name="username"
                      value={clickhouseConfig.username}
                      onChange={handleClickhouseConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  <div className="col-span-2">
                    <label className="block text-sm font-medium text-gray-700">JWT Token</label>
                    <input
                      type="password"
                      name="jwtToken"
                      value={clickhouseConfig.jwtToken}
                      onChange={handleClickhouseConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  {/* <div className="col-span-2">
                    <label className="inline-flex items-center">
                      <input
                        type="checkbox"
                        name="isHttps"
                        checked={clickhouseConfig.isHttps}
                        onChange={handleClickhouseConfigChange}
                        className="rounded border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                      />
                      <span className="ml-2">Use HTTPS</span>
                    </label>
                  </div> */}

                  <div className="col-span-2">
                    <label className="block text-sm font-medium text-gray-700">Target Table Name</label>
                    <input
                      type="text"
                      value={targetTableName}
                      onChange={(e) => setTargetTableName(e.target.value)}
                      placeholder="Enter target table name"
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                    <p className="mt-1 text-xs text-gray-500">
                      Enter the name of the table where data will be ingested. Table will be created if it doesn't exist.
                    </p>
                  </div>
                </div>
              )}

              {target === 'flatfile' && (
                <div className="grid grid-cols-2 gap-4">
                  <div className="col-span-2">
                    <label className="block text-sm font-medium text-gray-700">Output File Name/Path</label>
                    <input
                      type="text"
                      name="fileName"
                      value={flatFileConfig.fileName}
                      onChange={handleFlatFileConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700">Delimiter</label>
                    <input
                      type="text"
                      name="delimiter"
                      value={flatFileConfig.delimiter}
                      onChange={handleFlatFileConfigChange}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                    />
                  </div>

                  <div>
                    <label className="inline-flex items-center mt-6">
                      <input
                        type="checkbox"
                        name="hasHeader"
                        checked={flatFileConfig.hasHeader}
                        onChange={handleFlatFileConfigChange}
                        className="rounded border-gray-300 text-indigo-600 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                      />
                      <span className="ml-2">Include Header Row</span>
                    </label>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Column Selection */}
          {availableColumns.length > 0 && (
            <div className="mb-8">
              <div className="flex items-center justify-between mb-2">
                <h2 className="text-xl font-semibold">Column Selection</h2>
                <button
                  onClick={handleSelectAllColumns}
                  className="text-sm text-indigo-600 hover:text-indigo-900"
                >
                  {selectedColumns.length === availableColumns.length
                    ? 'Deselect All'
                    : 'Select All'}
                </button>
              </div>

              <div className="border rounded-md p-4 max-h-48 overflow-y-auto">
                <div className="grid grid-cols-3 gap-2">
                  {availableColumns.map((column) => {
                    const columnName = column.name || column;
                    return (
                      <div key={columnName} className="flex items-center">
                        <input
                          type="checkbox"
                          id={`col-${columnName}`}
                          checked={selectedColumns.includes(columnName)}
                          onChange={() => handleColumnToggle(columnName)}
                          className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                        />
                        <label htmlFor={`col-${columnName}`} className="ml-2 block text-sm text-gray-900 truncate">
                          {columnName}
                          {column.type && <span className="text-xs text-gray-500 ml-1">({column.type})</span>}
                        </label>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          )}

          {/* Preview and Ingestion Controls */}
          {availableColumns.length > 0 && (
            <div className="mb-8 grid grid-cols-2 gap-4">
              <div>
                <div className="flex items-center mb-2">
                  <label className="block text-sm font-medium text-gray-700 mr-2">Preview Limit:</label>
                  <input
                    type="number"
                    min="1"
                    max="1000"
                    value={previewLimit}
                    onChange={(e) => setPreviewLimit(parseInt(e.target.value, 10))}
                    className="w-24 rounded-md border-gray-300 shadow-sm focus:border-indigo-300 focus:ring focus:ring-indigo-200 focus:ring-opacity-50"
                  />
                </div>
                <button
                  onClick={handlePreviewData}
                  className="w-full inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  disabled={selectedColumns.length === 0 || status === 'connecting' || status === 'fetching' || status === 'ingesting'}
                >
                  Preview Data
                </button>
              </div>

              <div>
                <button
                  onClick={handleStartIngestion}
                  className="w-full inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
                  disabled={selectedColumns.length === 0 || status === 'connecting' || status === 'fetching' || status === 'ingesting' ||
                    (target === 'clickhouse' && !targetTableName) ||
                    (target === 'flatfile' && !flatFileConfig.fileName)}
                >
                  Start Ingestion
                </button>
              </div>
            </div>
          )}

          {/* Status and Progress */}
          <div className="mb-8">
            <h2 className="text-xl font-semibold mb-2">Status</h2>

            <div className={`p-4 rounded-md ${status === 'idle' ? 'bg-gray-100' :
                status === 'connecting' ? 'bg-blue-100' :
                  status === 'fetching' ? 'bg-blue-100' :
                    status === 'ingesting' ? 'bg-yellow-100' :
                      status === 'completed' ? 'bg-green-100' :
                        status === 'error' ? 'bg-red-100' : 'bg-gray-100'
              }`}>
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  {status === 'idle' && <span className="h-5 w-5 inline-block rounded-full bg-gray-400"></span>}
                  {(status === 'connecting' || status === 'fetching' || status === 'ingesting') && (
                    <svg className="animate-spin h-5 w-5 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                  )}
                  {status === 'completed' && (
                    <svg className="h-5 w-5 text-green-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                    </svg>
                  )}
                  {status === 'error' && (
                    <svg className="h-5 w-5 text-red-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                    </svg>
                  )}
                </div>
                <div className="ml-3">
                  <h3 className={`text-sm font-medium ${status === 'idle' ? 'text-gray-800' :
                      status === 'connecting' || status === 'fetching' ? 'text-blue-800' :
                        status === 'ingesting' ? 'text-yellow-800' :
                          status === 'completed' ? 'text-green-800' :
                            status === 'error' ? 'text-red-800' : 'text-gray-800'
                    }`}>
                    {status === 'idle' ? 'Ready' :
                      status === 'connecting' ? 'Connecting...' :
                        status === 'fetching' ? 'Fetching Data...' :
                          status === 'ingesting' ? 'Ingesting Data...' :
                            status === 'completed' ? 'Completed' :
                              status === 'error' ? 'Error' : 'Ready'}
                  </h3>
                  <div className="mt-1 text-sm text-gray-700">
                    {message}
                  </div>
                  {status === 'completed' && recordCount > 0 && (
                    <div className="mt-2 text-sm font-semibold text-green-700">
                      Total Records Processed: {recordCount}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Data Preview Table */}
          {previewData.length > 0 && (
            <div className="mb-8">
              <h2 className="text-xl font-semibold mb-2">Data Preview</h2>
              <div className="overflow-x-auto shadow rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      {Object.keys(previewData[0]).map((header) => (
                        <th
                          key={header}
                          scope="col"
                          className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                        >
                          {header}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {previewData.map((row, rowIndex) => (
                      <tr key={rowIndex} className={rowIndex % 2 === 0 ? 'bg-white' : 'bg-gray-50'}>
                        {Object.values(row).map((cell, cellIndex) => (
                          <td
                            key={cellIndex}
                            className="px-6 py-4 whitespace-nowrap text-sm text-gray-500"
                          >
                            {String(cell)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default App;