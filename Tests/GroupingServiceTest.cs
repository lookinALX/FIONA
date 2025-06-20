using FileSorter.Core.Services;

namespace Tests;

public class GroupingServiceTest
{
    private string _fileDirectoryTest = @"F:\projects\FIONA\Tests\";
    private string _fileDirectoryWithFiles = @"F:\projects\FIONA\Tests\testDir\";
    private GroupingService _groupingService = new GroupingService();
    
    [SetUp]
    public void Setup()
    {
        
    }

    [Test]
    public async Task Test_GetFilesToGroupAsync_ExistingDirectory()
    {
        // Arrange
        var fileDirectoryWithoutFiles = Path.Combine(_fileDirectoryTest, "testDirNoFiles");
        Directory.CreateDirectory(fileDirectoryWithoutFiles);
        
        // Act
        var resultWithFiles = await _groupingService.GetFilesToGroupAsync(_fileDirectoryWithFiles);
        var resultWithoutFiles = await _groupingService.GetFilesToGroupAsync(fileDirectoryWithoutFiles);
        
        var outputWithFiles = resultWithFiles.ToList();
        var outputWithoutFiles = resultWithoutFiles.ToList();
        
        // Assert
        Assert.That(outputWithFiles, Is.Not.Null);
        Assert.That(outputWithFiles.Count, Is.EqualTo(4));
        Assert.That(outputWithoutFiles, Is.Not.Null);
        Assert.That(outputWithoutFiles, Is.Empty);
        
        // Cleanup
        Directory.Delete(fileDirectoryWithoutFiles, true);
    }
}