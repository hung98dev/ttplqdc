using System.Collections;
using System.Linq;
using System.Threading.Tasks;
using UnityEngine;

public class Hud : MonoBehaviour
{
    private static int s_count;
    private UnityEvent m_evt;
    private Material m_mat;

    private void Awake()
    {
        m_mat = GetComponent<Renderer>().material;
        var cam = Camera.main;
        var go = GameObject.Find("root");
        SendMessage("Ping");
        Invoke("Later", 1f);
        StartCoroutine(Delay());
        Async();
        var t = Task.Run(() => 1);
        var asset = Resources.Load<Texture2D>("x").WaitForCompletion();
        var list = Enumerable.Range(0, 1).Where(x => x > 0);
        Debug.Log("hi");
        GC.Collect();
        var r = UnityEngine.Random.value;
        var r2 = new System.Random();
    }

    private IEnumerator Delay() { yield return null; }
    private async void Async() { await Task.Yield(); }
}
