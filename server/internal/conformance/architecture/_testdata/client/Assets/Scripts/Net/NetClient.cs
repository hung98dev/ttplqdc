using System.Threading.Tasks;

public class NetClient
{
    public void Go()
    {
        var t = Task.Run(() => 1);
    }
}
