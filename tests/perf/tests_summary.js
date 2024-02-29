const fs = require('fs')

module.exports = async ({ glob, core }) => {
    const globber = await glob.create(
        process.env['TEST_OUTPUT_FILE_PREFIX'] + '_metrics.json'
    )
    for await (const file of globber.globGenerator()) {
        const testSummary = JSON.parse(fs.readFileSync(file, 'utf8'))
        await core.summary
            .addHeading('Perf Test Results')
            .addTable(testSummary)
            .write()
    }
}
